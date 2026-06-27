package wecom

import (
	"encoding/xml"
	"fmt"
	"strings"
)

type EncryptedXMLBody struct {
	XMLName    xml.Name `xml:"xml"`
	ToUserName string   `xml:"ToUserName"`
	Encrypt    string   `xml:"Encrypt"`
	AgentID    string   `xml:"AgentID"`
}

type DecryptedXMLMessage struct {
	XMLName      xml.Name `xml:"xml"`
	ToUserName   string   `xml:"ToUserName"`
	FromUserName string   `xml:"FromUserName"`
	CreateTime   int64    `xml:"CreateTime"`
	MsgType      string   `xml:"MsgType"`
	Content      string   `xml:"Content"`
	MsgID        int64    `xml:"MsgId"`
	AgentID      int      `xml:"AgentID"`
	PicURL       string   `xml:"PicUrl"`
	MediaID      string   `xml:"MediaId"`
	Format       string   `xml:"Format"`
	Recognition  string   `xml:"Recognition"`
	ThumbMediaID string   `xml:"ThumbMediaId"`
	Event        string   `xml:"Event"`
	EventKey     string   `xml:"EventKey"`
}

type EncryptedXMLResponse struct {
	XMLName      xml.Name `xml:"xml"`
	Encrypt      string   `xml:"Encrypt"`
	MsgSignature string   `xml:"MsgSignature"`
	TimeStamp    string   `xml:"TimeStamp"`
	Nonce        string   `xml:"Nonce"`
}

func ParseEncryptedBody(data []byte) (EncryptedXMLBody, error) {
	var body EncryptedXMLBody
	if err := xml.Unmarshal(data, &body); err != nil {
		return EncryptedXMLBody{}, fmt.Errorf("parse encrypted xml: %w", err)
	}
	return body, nil
}

func ParseDecryptedMessage(data string) (DecryptedXMLMessage, error) {
	var msg DecryptedXMLMessage
	if err := xml.Unmarshal([]byte(data), &msg); err != nil {
		return DecryptedXMLMessage{}, fmt.Errorf("parse decrypted xml: %w", err)
	}
	return msg, nil
}

func XMLMessageToInbound(msg DecryptedXMLMessage) InboundMessage {
	return InboundMessage{
		ToUserName:   msg.ToUserName,
		FromUserName: msg.FromUserName,
		CreateTime:   msg.CreateTime,
		MsgType:      strings.ToLower(msg.MsgType),
		Content:      msg.Content,
		MsgID:        msg.MsgID,
		AgentID:      msg.AgentID,
		PicURL:       msg.PicURL,
		MediaID:      msg.MediaID,
		Format:       msg.Format,
		Recognition:  msg.Recognition,
		ThumbMediaID: msg.ThumbMediaID,
		EventType:    msg.Event,
		EventKey:     msg.EventKey,
	}
}

func BuildEncryptedResponse(encrypt, signature, timestamp, nonce string) ([]byte, error) {
	resp := EncryptedXMLResponse{
		Encrypt:      encrypt,
		MsgSignature: signature,
		TimeStamp:    timestamp,
		Nonce:        nonce,
	}
	return xml.Marshal(resp)
}
