package provider

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/dominicgisler/imap-spam-cleaner/imap"
	"github.com/dominicgisler/imap-spam-cleaner/logx"
)

func analysisPrompt(msg imap.Message, maxsize int) string {
	cont := messageContent(msg, maxsize)

	return fmt.Sprintf(
		"Analyze the following email for its spam potential.\n"+
			"Return a spam score between 0 and 100. Only answer with the number itself.\n\n"+
			"From: %s\nTo: %s\nDelivered-To: %s\nCc: %s\nBcc: %s\nSubject: %s\n\n"+
			"Content:\n%s",
		msg.From,
		msg.To,
		msg.DeliveredTo,
		msg.Cc,
		msg.Bcc,
		msg.Subject,
		cont,
	)
}

func messageContent(msg imap.Message, maxsize int) string {
	cont := ""
	contLen := 0

	for _, cnt := range msg.Contents {
		contLen += len(cnt)
		if contLen > maxsize {
			logx.Debugf("skipping bytes for message #%d (%s)", msg.UID, msg.Subject)
			break
		}
		cont += cnt + "\n"
	}

	return cont
}

func parseScore(resp string) (int, error) {
	i, err := strconv.ParseInt(strings.TrimSpace(resp), 10, 64)
	if err != nil {
		return 0, err
	}

	return int(i), nil
}