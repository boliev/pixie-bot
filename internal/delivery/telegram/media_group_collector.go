package telegram

import (
	"fmt"
	"sync"
	"time"
)

const collectWindow = 1500 * time.Millisecond

type mediaGroup struct {
	chatID  int64
	userID  int64
	fileIDs []string
	prompt  string
	timer   *time.Timer
}

// MediaGroupCollector buffers media group updates and flushes them after a short
// window following the last received item. Safe for concurrent use.
// NOTE: This in-memory implementation is suitable for a single long-polling instance.
// For horizontal scaling, replace with Redis or a distributed store.
type MediaGroupCollector struct {
	mu     sync.Mutex
	groups map[string]*mediaGroup
}

func NewMediaGroupCollector() *MediaGroupCollector {
	return &MediaGroupCollector{
		groups: make(map[string]*mediaGroup),
	}
}

// Add registers a photo from a media group update. After collectWindow of inactivity,
// flush is called with the accumulated data.
func (c *MediaGroupCollector) Add(
	chatID, userID int64,
	mediaGroupID, fileID, caption string,
	flush func(chatID, userID int64, fileIDs []string, prompt string),
) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := fmt.Sprintf("%d_%s", chatID, mediaGroupID)

	group, exists := c.groups[key]
	if !exists {
		group = &mediaGroup{
			chatID: chatID,
			userID: userID,
		}

		c.groups[key] = group
	}

	group.fileIDs = append(group.fileIDs, fileID)

	if caption != "" && group.prompt == "" {
		group.prompt = caption
	}

	if group.timer != nil {
		group.timer.Stop()
	}

	capturedKey := key

	capturedGroup := group

	group.timer = time.AfterFunc(collectWindow, func() {
		c.mu.Lock()
		delete(c.groups, capturedKey)
		c.mu.Unlock()
		flush(capturedGroup.chatID, capturedGroup.userID, capturedGroup.fileIDs, capturedGroup.prompt)
	})
}
