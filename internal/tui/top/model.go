package top

import (
	"context"
	"errors"
	"fmt"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/hnimtadd/hive/internal/transport/client"
	"github.com/hnimtadd/hive/internal/tui"
	"github.com/hnimtadd/hive/internal/tui/chat"
	"github.com/hnimtadd/hive/internal/tui/footer"
	"github.com/hnimtadd/hive/internal/tui/header"
	"github.com/hnimtadd/hive/internal/tui/help"
	"github.com/hnimtadd/hive/internal/tui/keys"
	"github.com/hnimtadd/hive/pkg/config"
	agentv1 "github.com/hnimtadd/hive/proto/agent/v1"
)

type model struct {
	cfg *config.Config

	mode   tui.Mode
	status tui.Status

	header        *header.Model
	chat          *chat.Model
	footer        *footer.Model
	help          *help.Model
	height, width int

	grpcClient *client.Client
	msgCh      chan tea.Msg // Channel for streaming messages
	ctx        context.Context
	cancel     context.CancelFunc

	conversationID        string
	responseCh            <-chan *agentv1.HiveSessionResponse
	streamListenerStarted bool
	requestToTask         map[string]string
	startedTasks          map[string]struct{}
}

type sessionUpdateMsg struct {
	update *agentv1.HiveSessionResponse
}

func newModel(cfg *config.Config) (*model, error) {
	header, err := header.NewModel(header.ModelOptions{
		Status: tui.StatusConnecting,
	})
	if err != nil {
		return nil, err
	}
	footer, err := footer.NewModel(footer.ModelOptions{
		Mode: tui.DefaultMode,
	})
	if err != nil {
		return nil, err
	}
	chat, err := chat.NewModel(chat.ModelOptions{})
	if err != nil {
		return nil, err
	}

	grpcClient, err := client.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create grpc client: %w", err)
	}

	helpModel := help.NewModel()

	ctx, cancel := context.WithCancel(context.Background())

	return &model{
		cfg:           cfg,
		mode:          tui.DefaultMode,
		header:        header,
		footer:        footer,
		chat:          chat,
		help:          helpModel,
		grpcClient:    grpcClient,
		msgCh:         make(chan tea.Msg, 100),
		ctx:           ctx,
		cancel:        cancel,
		requestToTask: map[string]string{},
		startedTasks:  map[string]struct{}{},
	}, nil
}

// cleanup closes gRPC connections and cancels context.
func (m *model) cleanup() {
	if m.cancel != nil {
		m.cancel()
	}
}

// Init implements [tea.Model].
func (m *model) Init() tea.Cmd {
	return m.waitForChannelMessage()
}

// waitForChannelMessage creates a command that waits for messages from msgCh.
func (m *model) waitForChannelMessage() tea.Cmd {
	return func() tea.Msg {
		select {
		case msg, ok := <-m.msgCh:
			if !ok {
				return nil
			}
			return msg
		case <-m.ctx.Done():
			return nil
		}
	}
}

// Update implements [tea.Model].
func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmd := []tea.Cmd{}
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.height, m.width = msg.Height, msg.Width
		m.header.Update(tea.WindowSizeMsg{
			Height: tui.HeaderHeight,
			Width:  m.width,
		})
		m.footer.Update(tea.WindowSizeMsg{
			Height: tui.FooterHeight,
			Width:  m.width,
		})

		m.chat.Update(tea.WindowSizeMsg{
			Height: msg.Height - tui.FooterHeight - tui.HeaderHeight - 2,
			Width:  m.width - 2,
		})

		m.help.Update(tea.WindowSizeMsg{
			Height: msg.Height - tui.FooterHeight - tui.HeaderHeight - 2,
			Width:  m.width - 2,
		})

	case tui.ErrorMsg, tui.InfoMsg:
		cmd = append(cmd, m.footer.Update(msg))

	case tui.ChangeModeMsg:
		if m.mode != tui.Mode(msg) {
			m.mode = tui.Mode(msg)
			switch m.mode {
			case tui.ModeNormal:
				m.chat.Update(tea.BlurMsg{})
			case tui.ModeInsert:
				m.chat.Update(tea.FocusMsg{})
			}
			m.footer.Update(msg)
			// Forward the mode change to chat so inputbar can update
			cmd = append(cmd, m.chat.Update(msg))
		}
	case tui.ChangeStatusMsg:
		if m.status != tui.Status(msg) {
			m.status = tui.Status(msg)
			m.header.Update(msg)
		}

	case tui.ToggleHelpMsg:
		m.help.Toggle()

	case chat.SendMessageMsg:
		cmd = append(cmd, m.executeTask(msg.Content))

	case chat.StreamStartMsg, chat.StreamChunkMsg, chat.StreamCompleteMsg, chat.FeedbackRequestMsg:
		// Forward streaming messages to chat model
		cmd = append(cmd, m.chat.Update(msg))
		// Chain the next channel read
		cmd = append(cmd, m.waitForChannelMessage())

	case chat.FeedbackResponseMsg:
		// Handle feedback response (send to server)
		if err := m.grpcClient.SendFeedback(m.ctx, msg.ConversationID, msg.TurnID, msg.Response); err != nil {
			cmd = append(cmd, tui.MsgCmd(tui.ErrorMsg(err)))
		}

	case sessionUpdateMsg:
		cmd = append(cmd, m.handleSessionUpdate(msg.update)...)
		cmd = append(cmd, m.waitForChannelMessage())

	case tea.KeyMsg:
		// Pressing any key makes any info/error message in the footer disappear.
		m.footer.ResetStatus()
		switch m.mode {
		case tui.ModeInsert:
			switch {
			case key.Matches(msg, keys.HiveKeys.Insert.Leave):
				return m, tui.MsgCmd(tui.ChangeModeMsg(tui.DefaultMode))
			default:
				cmd = append(cmd, m.chat.Update(msg))
			}
		case tui.ModeNormal:
			switch {
			case key.Matches(msg, keys.HiveKeys.Normal.Insert):
				return m, tui.MsgCmd(tui.ChangeModeMsg(tui.ModeInsert))
			case key.Matches(msg, keys.HiveKeys.Normal.Quit):
				m.cleanup()
				return m, tea.Quit
			case key.Matches(msg, keys.HiveKeys.Normal.Clear):
				return m, tui.MsgCmd(tui.ClearChatMsg{})
			case key.Matches(msg, keys.HiveKeys.Normal.Help):
				return m, tui.MsgCmd(tui.ToggleHelpMsg{})
			}
		}
	default:
		cmd = append(cmd, m.chat.Update(msg))
	}
	return m, tea.Batch(cmd...)
}

// View implements [tea.Model].
func (m *model) View() tea.View {
	var main string
	if m.help.IsActive() {
		main = tui.DefaultContainer.Width(m.width).PaddingBottom(2).AlignHorizontal(lipgloss.Center).Render(m.help.View())
	} else {
		main = tui.DefaultContainer.Width(m.width).PaddingBottom(2).AlignHorizontal(lipgloss.Center).Render(m.chat.View())
	}
	content := lipgloss.JoinVertical(lipgloss.Top,
		m.header.View().Content,
		main,
		m.footer.View().Content,
	)

	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

// executeTask sends a message to the gRPC server and handles the response stream.
func (m *model) executeTask(content string) tea.Cmd {
	if err := m.ensureConversation(); err != nil {
		return tui.MsgCmd(tui.ErrorMsg(err))
	}

	turnID, requestID, err := m.grpcClient.SendTurn(m.ctx, m.conversationID, content)
	if err != nil {
		return tui.MsgCmd(tui.ErrorMsg(err))
	}
	m.requestToTask[requestID] = turnID
	m.startedTasks[turnID] = struct{}{}

	select {
	case m.msgCh <- chat.StreamStartMsg{TaskID: turnID}:
	case <-m.ctx.Done():
		return tui.MsgCmd(tui.ErrorMsg(m.ctx.Err()))
	}

	return tui.NoopCmd
}

func (m *model) ensureConversation() error {
	if m.conversationID != "" && m.responseCh != nil {
		return nil
	}

	conversationID, responseCh, err := m.grpcClient.StartConversation(m.ctx, "")
	if err != nil {
		return err
	}

	m.conversationID = conversationID
	m.responseCh = responseCh
	if !m.streamListenerStarted {
		m.startStreamListener()
		m.streamListenerStarted = true
	}
	return nil
}

func (m *model) startStreamListener() {
	go func() {
		for {
			select {
			case <-m.ctx.Done():
				return

			case update, ok := <-m.responseCh:
				if !ok {
					return
				}
				select {
				case m.msgCh <- sessionUpdateMsg{update: update}:
				case <-m.ctx.Done():
					return
				}
			}
		}
	}()
}

func (m *model) handleSessionUpdate(update *agentv1.HiveSessionResponse) []tea.Cmd {
	cmds := []tea.Cmd{}
	if update == nil {
		return cmds
	}

	if createConv := update.GetCreateConversation(); createConv != nil {
		if m.conversationID == "" {
			m.conversationID = createConv.GetConversationId()
		}
		return cmds
	}

	if notification := update.GetNotification(); notification != nil {
		taskID := m.requestToTask[update.GetInReplyTo()]
		if errMsg := notification.GetError(); errMsg != "" {
			if taskID != "" {
				cmds = append(cmds, tui.MsgCmd(chat.StreamCompleteMsg{
					Success: false,
					Content: "",
					Error:   errors.New(errMsg),
					TaskID:  taskID,
				}))
				m.cleanupTaskTracking(taskID)
			} else {
				cmds = append(cmds, tui.MsgCmd(tui.ErrorMsg(errors.New(errMsg))))
			}
			return cmds
		}
		if info := notification.GetInfo(); info != "" {
			if taskID != "" {
				cmds = append(cmds, tui.MsgCmd(chat.StreamChunkMsg{
					Content: info,
					Status:  "info",
					TaskID:  taskID,
				}))
			} else {
				cmds = append(cmds, tui.MsgCmd(tui.InfoMsg(info)))
			}
			return cmds
		}
	}

	if turn := update.GetTurnResponse(); turn != nil {
		taskID := turn.GetTurnId()
		if taskID == "" {
			taskID = m.requestToTask[turn.GetRequestId()]
		}
		if taskID != "" {
			if _, started := m.startedTasks[taskID]; !started {
				m.startedTasks[taskID] = struct{}{}
				cmds = append(cmds, tui.MsgCmd(chat.StreamStartMsg{TaskID: taskID}))
			}
		}

		if progress := turn.GetUpdate(); progress != nil && taskID != "" {
			cmds = append(cmds, tui.MsgCmd(chat.StreamChunkMsg{
				Content: progress.GetContent(),
				Status:  "in_progress",
				TaskID:  taskID,
			}))
		}

		if completed := turn.GetCompleted(); completed != nil && taskID != "" {
			if success := completed.GetSuccess(); success != nil {
				cmds = append(cmds, tui.MsgCmd(chat.StreamCompleteMsg{
					Success: true,
					Content: success.GetContent(),
					Error:   nil,
					TaskID:  taskID,
				}))
				m.cleanupTaskTracking(taskID)
				return cmds
			}
			if failed := completed.GetFailed(); failed != nil {
				cmds = append(cmds, tui.MsgCmd(chat.StreamCompleteMsg{
					Success: false,
					Content: "",
					Error:   errors.New(failed.GetMessage()),
					TaskID:  taskID,
				}))
				m.cleanupTaskTracking(taskID)
				return cmds
			}
		}
	}

	if inputRequired := update.GetInputRequired(); inputRequired != nil {
		taskID := inputRequired.GetTurnId()
		if taskID != "" {
			if _, started := m.startedTasks[taskID]; !started {
				m.startedTasks[taskID] = struct{}{}
				cmds = append(cmds, tui.MsgCmd(chat.StreamStartMsg{TaskID: taskID}))
			}
		}
		cmds = append(cmds, tui.MsgCmd(chat.FeedbackRequestMsg{
			ConversationID: inputRequired.GetConversationId(),
			TurnID:         inputRequired.GetTurnId(),
			Question:       inputRequired.GetQuestion(),
		}))
	}

	return cmds
}

func (m *model) cleanupTaskTracking(taskID string) {
	delete(m.startedTasks, taskID)
	for reqID, currentTaskID := range m.requestToTask {
		if currentTaskID == taskID {
			delete(m.requestToTask, reqID)
		}
	}
}
