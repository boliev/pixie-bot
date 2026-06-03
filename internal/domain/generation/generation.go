package generation

import (
	"errors"
	"time"
)

const (
	MaxImages = 10
	MinImages = 1
)

var (
	ErrNotFound                   = errors.New("generation not found")
	ErrEmptyPrompt                = errors.New("prompt cannot be empty")
	ErrNoImages                   = errors.New("at least 1 image is required")
	ErrTooManyImages              = errors.New("maximum 10 images per generation")
	ErrMultipleImagesNotSupported = errors.New("multiple images not supported by current model")
)

type Status string

const (
	StatusProcessing Status = "processing"
	StatusDone       Status = "done"
	StatusFailed     Status = "failed"
)

type Generation struct {
	ID          int64
	UserID      int64
	Prompt      string
	Status      Status
	ImagesCount int
	Error       string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type GenerationImage struct {
	ID             int64
	GenerationID   int64
	TelegramFileID string
	Position       int
	CreatedAt      time.Time
}
