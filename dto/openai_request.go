// Copyright (c) 2025 Tethys Plex
//
// This file is part of Veloera.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.
package dto

import (
	"encoding/json"
	"strings"
)

type ResponseFormat struct {
	Type       string            `json:"type,omitempty"`
	JsonSchema *FormatJsonSchema `json:"json_schema,omitempty"`
}

type FormatJsonSchema struct {
	Description string `json:"description,omitempty"`
	Name        string `json:"name"`
	Schema      any    `json:"schema,omitempty"`
	Strict      any    `json:"strict,omitempty"`
}

type GeneralOpenAIRequest struct {
	Model               string            `json:"model,omitempty"`
	Messages            []Message         `json:"messages,omitempty"`
	Prompt              any               `json:"prompt,omitempty"`
	Prefix              any               `json:"prefix,omitempty"`
	Suffix              any               `json:"suffix,omitempty"`
	Stream              bool              `json:"stream,omitempty"`
	StreamOptions       *StreamOptions    `json:"stream_options,omitempty"`
	MaxTokens           uint              `json:"max_tokens,omitempty"`
	MaxCompletionTokens uint              `json:"max_completion_tokens,omitempty"`
	ReasoningEffort     string            `json:"reasoning_effort,omitempty"`
	Temperature         *float64          `json:"temperature,omitempty"`
	TopP                float64           `json:"top_p,omitempty"`
	TopK                int               `json:"top_k,omitempty"`
	Stop                any               `json:"stop,omitempty"`
	N                   int               `json:"n,omitempty"`
	Input               any               `json:"input,omitempty"`
	Instruction         string            `json:"instruction,omitempty"`
	Size                string            `json:"size,omitempty"`
	Functions           any               `json:"functions,omitempty"`
	FrequencyPenalty    float64           `json:"frequency_penalty,omitempty"`
	PresencePenalty     float64           `json:"presence_penalty,omitempty"`
	ResponseFormat      *ResponseFormat   `json:"response_format,omitempty"`
	EncodingFormat      any               `json:"encoding_format,omitempty"`
	Seed                float64           `json:"seed,omitempty"`
	Tools               []ToolCallRequest `json:"tools,omitempty"`
	ToolChoice          any               `json:"tool_choice,omitempty"`
	User                string            `json:"user,omitempty"`
	LogProbs            bool              `json:"logprobs,omitempty"`
	TopLogProbs         int               `json:"top_logprobs,omitempty"`
	Dimensions          int               `json:"dimensions,omitempty"`
	Modalities          any               `json:"modalities,omitempty"`
	Audio               any               `json:"audio,omitempty"`
	ExtraBody           any               `json:"extra_body,omitempty"`
}

type ToolCallRequest struct {
	ID       string          `json:"id,omitempty"`
	Type     string          `json:"type"`
	Function FunctionRequest `json:"function"`
}

type FunctionRequest struct {
	Description string `json:"description,omitempty"`
	Name        string `json:"name"`
	Parameters  any    `json:"parameters,omitempty"`
	Arguments   string `json:"arguments,omitempty"`
}

type StreamOptions struct {
	IncludeUsage bool `json:"include_usage,omitempty"`
}

func (r GeneralOpenAIRequest) GetMaxTokens() int {
	return int(r.MaxTokens)
}

func (r GeneralOpenAIRequest) ParseInput() []string {
	if r.Input == nil {
		return nil
	}
	var input []string
	switch r.Input.(type) {
	case string:
		input = []string{r.Input.(string)}
	case []any:
		input = make([]string, 0, len(r.Input.([]any)))
		for _, item := range r.Input.([]any) {
			if str, ok := item.(string); ok {
				input = append(input, str)
			}
		}
	}
	return input
}

type Message struct {
	Role                string          `json:"role"`
	Content             json.RawMessage `json:"content"`
	Name                *string         `json:"name,omitempty"`
	Prefix              *bool           `json:"prefix,omitempty"`
	ReasoningContent    string          `json:"reasoning_content,omitempty"`
	Reasoning           string          `json:"reasoning,omitempty"`
	ToolCalls           json.RawMessage `json:"tool_calls,omitempty"`
	ToolCallId          string          `json:"tool_call_id,omitempty"`
	parsedContent       []MediaContent
	parsedStringContent *string
}

type MediaContent struct {
	Type       string `json:"type"`
	Text       string `json:"text,omitempty"`
	ImageUrl   any    `json:"image_url,omitempty"`
	InputAudio any    `json:"input_audio,omitempty"`
	File       any    `json:"file,omitempty"`
}

func (m *MediaContent) GetImageMedia() *MessageImageUrl {
	if m.ImageUrl != nil {
		return m.ImageUrl.(*MessageImageUrl)
	}
	return nil
}

func (m *MediaContent) GetInputAudio() *MessageInputAudio {
	if m.InputAudio != nil {
		return m.InputAudio.(*MessageInputAudio)
	}
	return nil
}

func (m *MediaContent) GetFile() *MessageFile {
	if m.File != nil {
		return m.File.(*MessageFile)
	}
	return nil
}

type MessageImageUrl struct {
	Url      string `json:"url"`
	Detail   string `json:"detail"`
	MimeType string
}

func (m *MessageImageUrl) IsRemoteImage() bool {
	return strings.HasPrefix(m.Url, "http")
}

type MessageInputAudio struct {
	Data   string `json:"data"` //base64
	Format string `json:"format"`
}

type MessageFile struct {
	FileName string `json:"filename,omitempty"`
	FileData string `json:"file_data,omitempty"`
	FileId   string `json:"file_id,omitempty"`
}

const (
	ContentTypeText       = "text"
	ContentTypeImageURL   = "image_url"
	ContentTypeInputAudio = "input_audio"
	ContentTypeFile       = "file"
)

func (m *Message) GetPrefix() bool {
	if m.Prefix == nil {
		return false
	}
	return *m.Prefix
}

func (m *Message) SetPrefix(prefix bool) {
	m.Prefix = &prefix
}

func (m *Message) ParseToolCalls() []ToolCallRequest {
	if m.ToolCalls == nil {
		return nil
	}
	var toolCalls []ToolCallRequest
	if err := json.Unmarshal(m.ToolCalls, &toolCalls); err == nil {
		return toolCalls
	}
	return toolCalls
}

func (m *Message) SetToolCalls(toolCalls any) {
	toolCallsJson, _ := json.Marshal(toolCalls)
	m.ToolCalls = toolCallsJson
}

func (m *Message) StringContent() string {
	if m.parsedStringContent != nil {
		return *m.parsedStringContent
	}

	var stringContent string
	if err := json.Unmarshal(m.Content, &stringContent); err == nil {
		m.parsedStringContent = &stringContent
		return stringContent
	}

	contentStr := new(strings.Builder)
	arrayContent := m.ParseContent()
	for _, content := range arrayContent {
		if content.Type == ContentTypeText {
			contentStr.WriteString(content.Text)
		}
	}
	stringContent = contentStr.String()
	m.parsedStringContent = &stringContent

	return stringContent
}

func (m *Message) SetNullContent() {
	m.Content = nil
	m.parsedStringContent = nil
	m.parsedContent = nil
}

func (m *Message) SetStringContent(content string) {
	jsonContent, _ := json.Marshal(content)
	m.Content = jsonContent
	m.parsedStringContent = &content
	m.parsedContent = nil
}

func (m *Message) SetMediaContent(content []MediaContent) {
	jsonContent, _ := json.Marshal(content)
	m.Content = jsonContent
	m.parsedContent = nil
	m.parsedStringContent = nil
}

func (m *Message) IsStringContent() bool {
	if m.parsedStringContent != nil {
		return true
	}
	var stringContent string
	if err := json.Unmarshal(m.Content, &stringContent); err == nil {
		m.parsedStringContent = &stringContent
		return true
	}
	return false
}

func (m *Message) ParseContent() []MediaContent {
	if m.parsedContent != nil {
		return m.parsedContent
	}

	var contentList []MediaContent

	// 先尝试解析为字符串
	var stringContent string
	if err := json.Unmarshal(m.Content, &stringContent); err == nil {
		contentList = []MediaContent{{
			Type: ContentTypeText,
			Text: stringContent,
		}}
		m.parsedContent = contentList
		return contentList
	}

	// 尝试解析为数组
	var arrayContent []map[string]interface{}
	if err := json.Unmarshal(m.Content, &arrayContent); err == nil {
		for _, contentItem := range arrayContent {
			contentType, ok := contentItem["type"].(string)
			if !ok {
				continue
			}

			switch contentType {
			case ContentTypeText:
				if text, ok := contentItem["text"].(string); ok {
					contentList = append(contentList, MediaContent{
						Type: ContentTypeText,
						Text: text,
					})
				}

			case ContentTypeImageURL:
				imageUrl := contentItem["image_url"]
				temp := &MessageImageUrl{
					Detail: "high",
				}
				switch v := imageUrl.(type) {
				case string:
					temp.Url = v
				case map[string]interface{}:
					url, ok1 := v["url"].(string)
					detail, ok2 := v["detail"].(string)
					if ok2 {
						temp.Detail = detail
					}
					if ok1 {
						temp.Url = url
					}
				}
				contentList = append(contentList, MediaContent{
					Type:     ContentTypeImageURL,
					ImageUrl: temp,
				})

			case ContentTypeInputAudio:
				if audioData, ok := contentItem["input_audio"].(map[string]interface{}); ok {
					data, ok1 := audioData["data"].(string)
					format, ok2 := audioData["format"].(string)
					if ok1 && ok2 {
						temp := &MessageInputAudio{
							Data:   data,
							Format: format,
						}
						contentList = append(contentList, MediaContent{
							Type:       ContentTypeInputAudio,
							InputAudio: temp,
						})
					}
				}
			case ContentTypeFile:
				if fileData, ok := contentItem["file"].(map[string]interface{}); ok {
					fileId, ok3 := fileData["file_id"].(string)
					if ok3 {
						contentList = append(contentList, MediaContent{
							Type: ContentTypeFile,
							File: &MessageFile{
								FileId: fileId,
							},
						})
					} else {
						fileName, ok1 := fileData["filename"].(string)
						fileDataStr, ok2 := fileData["file_data"].(string)
						if ok1 && ok2 {
							contentList = append(contentList, MediaContent{
								Type: ContentTypeFile,
								File: &MessageFile{
									FileName: fileName,
									FileData: fileDataStr,
								},
							})
						}
					}
				}
			}
		}
	}

	if len(contentList) > 0 {
		m.parsedContent = contentList
	}
	return contentList
}

// ConvertArrayContentToString converts array content to single string if all items are text
// Returns true if conversion was performed, false otherwise
func (m *Message) ConvertArrayContentToString() bool {
	// Parse the content as array once
	var arrayContent []map[string]interface{}
	if err := json.Unmarshal(m.Content, &arrayContent); err != nil {
		// Not array format, no conversion needed
		return false
	}

	// Check if all items are text type and collect text content
	var textBuilder strings.Builder
	for i, contentItem := range arrayContent {
		contentType, ok := contentItem["type"].(string)
		if !ok || contentType != ContentTypeText {
			// Not all text content, no conversion needed
			return false
		}

		if text, ok := contentItem["text"].(string); ok {
			if i > 0 {
				textBuilder.WriteString(" ")
			}
			textBuilder.WriteString(text)
		}
	}

	// All items are text, perform conversion
	m.SetStringContent(textBuilder.String())
	return true
}

type OpenAIResponsesRequest struct {
	Model              string               `json:"model"`
	Input              json.RawMessage      `json:"input,omitempty"`
	Include            json.RawMessage      `json:"include,omitempty"`
	Instructions       json.RawMessage      `json:"instructions,omitempty"`
	MaxOutputTokens    uint                 `json:"max_output_tokens,omitempty"`
	Metadata           json.RawMessage      `json:"metadata,omitempty"`
	ParallelToolCalls  bool                 `json:"parallel_tool_calls,omitempty"`
	PreviousResponseID string               `json:"previous_response_id,omitempty"`
	Reasoning          *Reasoning           `json:"reasoning,omitempty"`
	ServiceTier        string               `json:"service_tier,omitempty"`
	Store              bool                 `json:"store,omitempty"`
	Stream             bool                 `json:"stream,omitempty"`
	Temperature        float64              `json:"temperature,omitempty"`
	Text               json.RawMessage      `json:"text,omitempty"`
	ToolChoice         json.RawMessage      `json:"tool_choice,omitempty"`
	Tools              []ResponsesToolsCall `json:"tools,omitempty"`
	TopP               float64              `json:"top_p,omitempty"`
	Truncation         string               `json:"truncation,omitempty"`
	User               string               `json:"user,omitempty"`
}

type Reasoning struct {
	Effort  string `json:"effort,omitempty"`
	Summary string `json:"summary,omitempty"`
}

type ResponsesToolsCall struct {
	Type string `json:"type"`
	// Web Search
	UserLocation      json.RawMessage `json:"user_location,omitempty"`
	SearchContextSize string          `json:"search_context_size,omitempty"`
	// File Search
	VectorStoreIds []string        `json:"vector_store_ids,omitempty"`
	MaxNumResults  uint            `json:"max_num_results,omitempty"`
	Filters        json.RawMessage `json:"filters,omitempty"`
	// Computer Use
	DisplayWidth  uint   `json:"display_width,omitempty"`
	DisplayHeight uint   `json:"display_height,omitempty"`
	Environment   string `json:"environment,omitempty"`
	// Function
	Name        string          `json:"name,omitempty"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}
