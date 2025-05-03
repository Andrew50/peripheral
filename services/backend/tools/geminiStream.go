package tools

import (
	"backend/utils"
	"context"
	"fmt"

	"google.golang.org/genai"
)

// StreamGeminiText sends each text chunk to the user's WS channel
func StreamGeminiText(
	ctx context.Context,
	conn *utils.Conn,
	userID int,
	model string,
	systemPrompt string,
	promptParts []*genai.Content, // the user+context parts you already build
	config *genai.GenerateContentConfig, // can contain tools/thinking cfg (reuse your code)
) (final string, err error) {
	apiKey, err := conn.GetGeminiKey()
	if err != nil {
		return "", err
	}
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return "", err
	}
	for stream, err := range client.Models.GenerateContentStream(ctx, model, promptParts, config) {
		if err != nil {
			return "", err
		}
		fmt.Println("stream ", stream)
		fmt.Println(stream.Candidates[0].Content.Parts[0].Text)
	}
	return "", nil
	/*
		// Unique id so the FE can stitch the chunks together
		msgID := uuid.NewString()
		var b strings.Builder

		for {
			resp, err := stream.Recv()
			if err == io.EOF {
				break // stream finished
			}
			if err != nil {
				return "", err
			}
			if len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil {
				continue
			}
			for _, part := range resp.Candidates[0].Content.Parts {
				if txt := part.GetText(); txt != "" {
					b.WriteString(txt)
					socket.SendLLMChunk(userID, msgID, txt, false) // <= new helper below
				}
			}
		}

		final = b.String()
		socket.SendLLMChunk(userID, msgID, "", true) // mark as finished
		return final, nil */
}
