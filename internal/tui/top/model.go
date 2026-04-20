package top

import (
	"context"
	"errors"
	"fmt"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/hnimtadd/hive/internal/transport/client"
	"github.com/hnimtadd/hive/internal/tui"
	"github.com/hnimtadd/hive/internal/tui/chat"
	"github.com/hnimtadd/hive/internal/tui/footer"
	"github.com/hnimtadd/hive/internal/tui/header"
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
	chat, err := chat.NewModel(chat.ModelOptions{
		OnSendMessage: func(_ string) {
		},
	})
	if err != nil {
		return nil, err
	}

	grpcClient, err := client.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create grpc client: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &model{
		cfg:        cfg,
		mode:       tui.DefaultMode,
		header:     header,
		footer:     footer,
		chat:       chat,
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
	// Start the message pump ticker
	return tea.Tick(time.Millisecond*10, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

type tickMsg time.Time

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
		}
	case tui.ChangeStatusMsg:
		if m.status != tui.Status(msg) {
			m.status = tui.Status(msg)
			m.header.Update(msg)
		}

	case tui.ClearChatMsg:
		m.chat.ClearMessages()

	case chat.SendMessageMsg:
		cmd = append(cmd, m.executeTask(msg.Content))

	case chat.StreamChunkMsg, chat.StreamCompleteMsg, chat.FeedbackRequestMsg, chat.ResponseMsg:
		// Forward streaming messages to chat model
		cmd = append(cmd, m.chat.Update(msg))

	case tickMsg:
		// Poll msgCh for any pending messages
		select {
		case msg, ok := <-m.msgCh:
			if ok {
				// Forward the message to Update to be processed
				cmd = append(cmd, func() tea.Msg { return msg })
			}
		default:
			// No message ready
		}
		// Continue ticking if context is active
		if m.ctx.Err() == nil {
			cmd = append(cmd, tea.Tick(time.Millisecond*10, func(t time.Time) tea.Msg {
				return tickMsg(t)
			}))
		}

	case tea.KeyMsg:
		// Pressing any key makes any info/error message in the footer disappear.
		m.footer.ResetStatus()
		switch m.mode {
		case tui.ModeInsert:
			switch {
			case key.Matches(msg, keys.Insert.Leave):
				return m, tui.MsgCmd(tui.ChangeModeMsg(tui.DefaultMode))
			default:
				cmd = append(cmd, m.chat.Update(msg))
			}
		case tui.ModeNormal:
			switch {
			case key.Matches(msg, keys.Normal.Insert):
				return m, tui.MsgCmd(tui.ChangeModeMsg(tui.ModeInsert))
			case key.Matches(msg, keys.Normal.Quit):
				m.cleanup()
				return m, tea.Quit
			case key.Matches(msg, keys.Normal.Clear):
				return m, tui.MsgCmd(tui.ClearChatMsg{})
			}
		}
	default:
		cmd = append(cmd, m.chat.Update(msg))
	}
	return m, tea.Batch(cmd...)
}

// View implements [tea.Model].
func (m *model) View() tea.View {
	v := tea.NewView(lipgloss.JoinVertical(lipgloss.Top,
		m.header.View().Content,
		tui.DefaultContainer.Width(m.width).PaddingBottom(2).AlignHorizontal(lipgloss.Center).Render(m.chat.View()),
		m.footer.View().Content,
	))

	v.AltScreen = true
	return v
}

// executeTask sends a message to the gRPC server and handles the response stream.
func (m *model) executeTask(content string) tea.Cmd {
	return m.listenToStream(content)
}

// listenToStream creates a command that executes the task and listens to streaming responses
func (m *model) listenToStream(content string) tea.Cmd {
	responseCh := make(chan *agentv1.ExecuteTaskResponse, 100)

	go func() {
		defer close(responseCh)
		err := m.grpcClient.ExecuteTaskWithChannel(m.ctx, content, responseCh)
		if err != nil {
			select {
			case m.msgCh <- chat.ResponseMsg{
				Content: "",
				Error:   err,
			}:
			case <-m.ctx.Done():
				return
			}
		}
	}()

	go func() {
		isFirst := true
		for update := range responseCh {
			switch msg := update.GetPayload().(type) {
			case *agentv1.ExecuteTaskResponse_Update:
				select {
				case m.msgCh <- chat.StreamChunkMsg{
					Content: msg.Update.GetContent(),
					Status:  msg.Update.GetStatus(),
					IsFirst: isFirst,
				}:
				case <-m.ctx.Done():
					return
				}
				isFirst = false

			case *agentv1.ExecuteTaskResponse_Success:
				select {
				case m.msgCh <- chat.StreamCompleteMsg{
					Success: true,
					Content: msg.Success.GetContent(),
					Error:   nil,
				}:
				case <-m.ctx.Done():
					return
				}

			case *agentv1.ExecuteTaskResponse_Error:
				select {
				case m.msgCh <- chat.StreamCompleteMsg{
					Success: false,
					Content: "",
					Error:   errors.New(msg.Error.GetMessage()),
				}:
				case <-m.ctx.Done():
					return
				}

			case *agentv1.ExecuteTaskResponse_Feedback:
				select {
				case m.msgCh <- chat.FeedbackRequestMsg{
					Question: msg.Feedback.GetQuestion(),
				}:
				case <-m.ctx.Done():
					return
				}
			}
		}
	}()

	// The ticker in Init() will poll msgCh continuously
	return nil
}
