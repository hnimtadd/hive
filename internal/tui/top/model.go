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
		cfg:        cfg,
		mode:       tui.DefaultMode,
		header:     header,
		footer:     footer,
		chat:       chat,
		help:       helpModel,
		grpcClient: grpcClient,
		msgCh:      make(chan tea.Msg, 100),
		ctx:        ctx,
		cancel:     cancel,
	}, nil
}

// cleanup closes gRPC connections and cancels context.
func (m *model) cleanup() {
	if m.cancel != nil {
		m.cancel()
	}
	close(m.msgCh)
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
		if err := m.grpcClient.SendFeedback(m.ctx, msg.TaskID, msg.Response); err != nil {
			cmd = append(cmd, tui.MsgCmd(tui.ErrorMsg(err)))
		}

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
	return m.listenToStream(content)
}

// listenToStream creates a command that executes the task and listens to streaming responses
func (m *model) listenToStream(content string) tea.Cmd {
	responseCh, err := m.grpcClient.ExecuteTaskWithChannel(m.ctx, content)
	if err != nil {
		return tui.MsgCmd(tui.ErrorMsg(err))
	}

	go func() {
		taskID := ""
		streamStarted := false
		for {
			select {
			case <-m.ctx.Done():
				return

			case update, ok := <-responseCh:
				if !ok {
					return
				}

				switch msg := update.GetPayload().(type) {
				case *agentv1.ExecuteTaskResponse_Ack:
					taskID = msg.Ack.GetTaskId()
					streamStarted = true
					select {
					case m.msgCh <- chat.StreamStartMsg{
						TaskID: taskID,
					}:
					case <-m.ctx.Done():
						return
					}

				case *agentv1.ExecuteTaskResponse_Update:
					if !streamStarted {
						streamStarted = true
						select {
						case m.msgCh <- chat.StreamStartMsg{
							TaskID: taskID,
						}:
						case <-m.ctx.Done():
							return
						}
					}
					select {
					case m.msgCh <- chat.StreamChunkMsg{
						Content: msg.Update.GetContent(),
						Status:  msg.Update.GetStatus(),
						TaskID:  taskID,
					}:
					case <-m.ctx.Done():
						return
					}

				case *agentv1.ExecuteTaskResponse_Success:
					if !streamStarted {
						streamStarted = true
						select {
						case m.msgCh <- chat.StreamStartMsg{
							TaskID: taskID,
						}:
						case <-m.ctx.Done():
							return
						}
					}
					select {
					case m.msgCh <- chat.StreamCompleteMsg{
						Success: true,
						Content: msg.Success.GetContent(),
						Error:   nil,
						TaskID:  taskID,
					}:
					case <-m.ctx.Done():
						return
					}

				case *agentv1.ExecuteTaskResponse_Error:
					if !streamStarted {
						streamStarted = true
						select {
						case m.msgCh <- chat.StreamStartMsg{
							TaskID: taskID,
						}:
						case <-m.ctx.Done():
							return
						}
					}
					select {
					case m.msgCh <- chat.StreamCompleteMsg{
						Success: false,
						Content: "",
						Error:   errors.New(msg.Error.GetMessage()),
						TaskID:  taskID,
					}:
					case <-m.ctx.Done():
						return
					}

				case *agentv1.ExecuteTaskResponse_Feedback:
					select {
					case m.msgCh <- chat.FeedbackRequestMsg{
						Question: msg.Feedback.GetQuestion(),
						TaskID:   taskID,
					}:
					case <-m.ctx.Done():
						return
					}
				}
			}
		}
	}()

	// The ticker in Init() will poll msgCh continuously
	return tui.NoopCmd
}
