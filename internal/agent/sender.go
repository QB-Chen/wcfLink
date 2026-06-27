package agent

import (
	"context"
	"fmt"
)

type ILinkSender interface {
	SendText(ctx context.Context, accountID, toUserID, text, contextToken string) error
}

type WeComSender interface {
	SendText(ctx context.Context, corpID, corpSecret string, agentID int, toUser, text string) error
}

type ILinkSessionInfo struct {
	AccountID    string
	ContextToken string
}

type WeComSessionInfo struct {
	CorpID     string
	CorpSecret string
	AgentID    int
}

type MultiChannelSender struct {
	ilinkSender ILinkSender
	wecomSender WeComSender
	ilinkInfo   func(session SessionKey) (ILinkSessionInfo, error)
	wecomInfo   func(session SessionKey) (WeComSessionInfo, error)
}

func NewMultiChannelSender(
	ilinkSender ILinkSender,
	wecomSender WeComSender,
	ilinkInfo func(SessionKey) (ILinkSessionInfo, error),
	wecomInfo func(SessionKey) (WeComSessionInfo, error),
) *MultiChannelSender {
	return &MultiChannelSender{
		ilinkSender: ilinkSender,
		wecomSender: wecomSender,
		ilinkInfo:   ilinkInfo,
		wecomInfo:   wecomInfo,
	}
}

func (s *MultiChannelSender) SendText(ctx context.Context, session SessionKey, text string) error {
	switch session.ChannelType {
	case "ilink":
		info, err := s.ilinkInfo(session)
		if err != nil {
			return fmt.Errorf("resolve ilink session info: %w", err)
		}
		return s.ilinkSender.SendText(ctx, info.AccountID, session.UserID, text, info.ContextToken)
	case "wecom":
		info, err := s.wecomInfo(session)
		if err != nil {
			return fmt.Errorf("resolve wecom session info: %w", err)
		}
		return s.wecomSender.SendText(ctx, info.CorpID, info.CorpSecret, info.AgentID, session.UserID, text)
	default:
		return fmt.Errorf("unsupported channel type: %s", session.ChannelType)
	}
}
