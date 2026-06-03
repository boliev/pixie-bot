package openai

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"
)

const defaultBaseURL = "https://api.openai.com/v1"

type ImageEditor struct {
	apiKey     string
	model      string
	baseURL    string
	httpClient *http.Client
}

func NewImageEditor(apiKey, model string) *ImageEditor {
	return &ImageEditor{
		apiKey:  apiKey,
		model:   model,
		baseURL: defaultBaseURL,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// EditImages sends images and a prompt to OpenAI Images API and returns the edited image bytes.
// Uses multipart/form-data with image[] fields for multi-image support (gpt-image-1).
// TODO: verify current OpenAI API docs if the model changes — field naming for multiple images
// may differ between models (e.g., dall-e-2 supports only single image).
func (e *ImageEditor) EditImages(ctx context.Context, images [][]byte, prompt string) ([]byte, error) {
	if len(images) == 0 {
		return nil, errors.New("no images provided")
	}

	body, contentType, err := e.buildMultipartBody(images, prompt)
	if err != nil {
		return nil, err
	}

	respBody, err := e.doEditRequest(ctx, body, contentType)
	if err != nil {
		return nil, err
	}

	return e.parseEditResponse(respBody)
}

func (e *ImageEditor) buildMultipartBody(images [][]byte, prompt string) (*bytes.Buffer, string, error) {
	var body bytes.Buffer

	w := multipart.NewWriter(&body)

	for i, img := range images {
		fieldName := "image"
		if len(images) > 1 {
			fieldName = "image[]"
		}

		part, err := w.CreateFormFile(fieldName, fmt.Sprintf("image_%d.png", i))
		if err != nil {
			return nil, "", fmt.Errorf("create form file: %w", err)
		}

		if _, err = part.Write(img); err != nil {
			return nil, "", fmt.Errorf("write image bytes: %w", err)
		}
	}

	if err := w.WriteField("prompt", prompt); err != nil {
		return nil, "", fmt.Errorf("write prompt field: %w", err)
	}

	if err := w.WriteField("model", e.model); err != nil {
		return nil, "", fmt.Errorf("write model field: %w", err)
	}

	if err := w.WriteField("n", "1"); err != nil {
		return nil, "", fmt.Errorf("write n field: %w", err)
	}

	if err := w.WriteField("response_format", "b64_json"); err != nil {
		return nil, "", fmt.Errorf("write response_format field: %w", err)
	}

	w.Close()

	return &body, w.FormDataContentType(), nil
}

func (e *ImageEditor) doEditRequest(ctx context.Context, body *bytes.Buffer, contentType string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.baseURL+"/images/edits", body)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+e.apiKey)
	req.Header.Set("Content-Type", contentType)

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}

	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openai API error %d: %s", resp.StatusCode, respBody)
	}

	return respBody, nil
}

func (e *ImageEditor) parseEditResponse(respBody []byte) ([]byte, error) {
	var result struct {
		Data []struct {
			B64JSON string `json:"b64_json"`
		} `json:"data"`
	}

	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	if len(result.Data) == 0 || result.Data[0].B64JSON == "" {
		return nil, errors.New("empty response from openai")
	}

	imgBytes, err := base64.StdEncoding.DecodeString(result.Data[0].B64JSON)
	if err != nil {
		return nil, fmt.Errorf("decode base64 image: %w", err)
	}

	return imgBytes, nil
}
