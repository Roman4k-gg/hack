package chat

import (
	"encoding/json"
	"strings"
)

type Collector struct {
	text          strings.Builder
	toolCalls     map[string]ToolCall
	toolOrder     []string
	citations     map[string]Citation
	citationOrder []string
	dataLines     []string
	eventName     string
}

func NewCollector() *Collector {
	return &Collector{
		toolCalls: map[string]ToolCall{},
		citations: map[string]Citation{},
	}
}

func (c *Collector) Feed(chunk []byte) {
	for _, line := range strings.Split(string(chunk), "\n") {
		line = strings.TrimRight(line, "\r")

		if line == "" {
			c.dispatch()
			continue
		}

		if strings.HasPrefix(line, ":") {
			continue
		}

		field, value, found := strings.Cut(line, ":")
		if !found {
			continue
		}

		value = strings.TrimPrefix(value, " ")

		switch field {
		case "event":
			c.eventName = value
		case "data":
			c.dataLines = append(c.dataLines, value)
		}
	}
}

func (c *Collector) dispatch() {
	if len(c.dataLines) == 0 {
		c.eventName = ""
		return
	}

	raw := strings.Join(c.dataLines, "\n")
	c.dataLines = nil
	c.eventName = ""

	var event struct {
		Type     string `json:"type"`
		Text     string `json:"text"`
		ToolCall *struct {
			ID     string `json:"id"`
			Name   string `json:"name"`
			Status string `json:"status"`
		} `json:"toolCall"`
		Citation *struct {
			ID    string `json:"id"`
			Title string `json:"title"`
			URL   string `json:"url"`
		} `json:"citation"`
	}

	if err := json.Unmarshal([]byte(raw), &event); err != nil {
		return
	}

	switch event.Type {
	case "delta":
		c.text.WriteString(event.Text)
	case "tool_call":
		if event.ToolCall == nil {
			return
		}

		if _, ok := c.toolCalls[event.ToolCall.ID]; !ok {
			c.toolOrder = append(c.toolOrder, event.ToolCall.ID)
		}

		c.toolCalls[event.ToolCall.ID] = ToolCall{
			ID:     event.ToolCall.ID,
			Name:   event.ToolCall.Name,
			Status: event.ToolCall.Status,
		}
	case "citation":
		if event.Citation == nil {
			return
		}

		if _, ok := c.citations[event.Citation.ID]; !ok {
			c.citationOrder = append(c.citationOrder, event.Citation.ID)
		}

		c.citations[event.Citation.ID] = Citation{
			ID:    event.Citation.ID,
			Title: event.Citation.Title,
			URL:   event.Citation.URL,
		}
	}
}

func (c *Collector) Result() (string, []ToolCall, []Citation) {
	toolCalls := make([]ToolCall, 0, len(c.toolOrder))
	for _, id := range c.toolOrder {
		toolCalls = append(toolCalls, c.toolCalls[id])
	}

	citations := make([]Citation, 0, len(c.citationOrder))
	for _, id := range c.citationOrder {
		citations = append(citations, c.citations[id])
	}

	return c.text.String(), toolCalls, citations
}
