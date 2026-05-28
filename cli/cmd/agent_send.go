package cmd

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/WuErPing/solo/cli/internal/output"
	"github.com/WuErPing/solo/protocol"
)

var (
	agentSendNoWait bool
	agentSendImage  []string
)

var agentSendCmd = &cobra.Command{
	Use:   "send <id> <message>",
	Short: "Send a message to an agent",
	Args:  cobra.ExactArgs(2),
	RunE:  runAgentSend,
}

func init() {
	agentSendCmd.Flags().BoolVar(&agentSendNoWait, "no-wait", false, "Don't wait for response")
	agentSendCmd.Flags().StringArrayVar(&agentSendImage, "image", nil, "Attach image(s)")
	agentCmd.AddCommand(agentSendCmd)
}

func runAgentSend(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	c, err := newClient(ctx, flagHost)
	if err != nil {
		return err
	}
	defer c.Close()

	agentID, err := fetchAndResolveAgentID(ctx, c, args[0])
	if err != nil {
		return err
	}
	message := args[1]

	req := &protocol.SendAgentMessageRequest{
		Type:    "send_agent_message_request",
		AgentID: agentID,
		Text:    message,
	}

	// Load images if provided
	if len(agentSendImage) > 0 {
		images, err := loadImages(agentSendImage)
		if err != nil {
			return err
		}
		req.Images = images
	}

	resp, err := c.Request(ctx, req)
	if err != nil {
		return fmt.Errorf("send message: %w", err)
	}

	if isRPCError(resp) {
		return &output.CommandError{Code: "SEND_FAILED", Message: extractRPCError(resp)}
	}

	opts := getOutputOpts(flagFormat, flagJSON, flagQuiet, flagNoHeaders, flagNoColor)
	if opts.Format == output.FormatJSON || opts.Format == output.FormatYAML {
		return output.Render(cmdStdout, output.SingleResult(map[string]string{
			"agentId": agentID,
			"status":  "sent",
		}, nil), opts)
	}

	fmt.Fprintf(cmdStdout, "Message sent to agent %s\n", shortenID(agentID))
	return nil
}

func loadImages(paths []string) ([]protocol.ImageAttachment, error) {
	var images []protocol.ImageAttachment
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, fmt.Errorf("read image %s: %w", p, err)
		}
		mimeType := detectMIME(p)
		encoded := base64.StdEncoding.EncodeToString(data)
		images = append(images, protocol.ImageAttachment{
			Data:     encoded,
			MimeType: mimeType,
		})
	}
	return images, nil
}

func detectMIME(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".svg":
		return "image/svg+xml"
	default:
		return "application/octet-stream"
	}
}
