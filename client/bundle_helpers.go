package youtrack

import (
	"bytes"
	"context"

	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type pageFetchConfig struct {
	PathFormat string
	HostURL    string
	APIPath    string
	Fields     string
	PageSize   int
	ErrCreate  string
	ErrFetch   string
}

type idFetchConfig struct {
	PathFormat string
	HostURL    string
	APIPath    string
	Fields     string
	ErrCreate  string
	ErrFetch   string
	ErrDecode  string
}

type createConfig struct {
	PathFormat string
	HostURL    string
	APIPath    string
	Fields     string
	ErrMarshal string
	ErrCreate  string
	ErrFetch   string
	ErrDecode  string
}

type updateConfig struct {
	PathFormat string
	HostURL    string
	APIPath    string
	Fields     string
	ErrMarshal string
	ErrCreate  string
	ErrFetch   string
	ErrDecode  string
}

type deleteConfig struct {
	HostURL   string
	APIPath   string
	ErrCreate string
	ErrFetch  string
}

// lookupByNamePaginated pages through all results looking for an exact match on
// targetName. An exact match on any page wins immediately. A case-insensitive
// match is only returned as a fallback once every page has been scanned and no
// exact match was found anywhere, so an exact match on a later page is never
// shadowed by a case-insensitive match on an earlier one.
func lookupByNamePaginated[T any](
	ctx context.Context,
	pageSize int,
	targetName string,
	fetchPage func(context.Context, int) ([]T, error),
	nameOf func(T) string,
) (*T, error) {
	var fuzzyMatch *T

	for skip := 0; ; skip += pageSize {
		items, err := fetchPage(ctx, skip)
		if err != nil {
			return nil, err
		}

		exact, fuzzy := findByName(items, targetName, nameOf)
		if exact != nil {
			return exact, nil
		}
		if fuzzyMatch == nil {
			fuzzyMatch = fuzzy
		}

		if len(items) < pageSize {
			break
		}
	}

	return fuzzyMatch, nil
}

// findByName scans a single page for an exact match on targetName, and separately
// tracks the first case-insensitive match as a fallback candidate.
func findByName[T any](items []T, targetName string, nameOf func(T) string) (exact *T, fuzzy *T) {
	for i := range items {
		name := nameOf(items[i])
		if name == targetName {
			return &items[i], nil
		}

		if fuzzy == nil && strings.EqualFold(name, targetName) {
			fuzzy = &items[i]
		}
	}

	return nil, fuzzy
}

func fetchAndDecodePage[T any](
	ctx context.Context,
	client *Client,
	cfg pageFetchConfig,
	skip int,
	decode func([]byte) ([]T, error),
) ([]T, error) {
	req, err := http.NewRequestWithContext(ctx, httpMethodGet,
		fmt.Sprintf(cfg.PathFormat, cfg.HostURL, cfg.APIPath, cfg.Fields, cfg.PageSize, skip), nil)
	if err != nil {
		return nil, fmt.Errorf(cfg.ErrCreate, err)
	}

	body, err := client.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf(cfg.ErrFetch, err)
	}

	return decode(body)
}

func decodeBundleList[T any](body []byte, errMsg string) ([]T, error) {
	var items []T
	if err := json.Unmarshal(body, &items); err == nil {
		return items, nil
	}

	var wrapped struct {
		Bundles []T `json:"bundles"`
	}
	if err := json.Unmarshal(body, &wrapped); err != nil {
		return nil, fmt.Errorf(errMsg, err)
	}

	return wrapped.Bundles, nil
}

func fetchAndDecodeByID[T any](
	ctx context.Context,
	client *Client,
	cfg idFetchConfig,
	id string,
) (*T, error) {
	req, err := http.NewRequestWithContext(ctx, httpMethodGet,
		fmt.Sprintf(cfg.PathFormat, cfg.HostURL, cfg.APIPath, id, cfg.Fields), nil)
	if err != nil {
		return nil, fmt.Errorf(cfg.ErrCreate, err)
	}

	body, err := client.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf(cfg.ErrFetch, err)
	}

	var item T
	if err := json.Unmarshal(body, &item); err != nil {
		return nil, fmt.Errorf(cfg.ErrDecode, err)
	}

	return &item, nil
}

func createAndDecode[T any](
	ctx context.Context,
	client *Client,
	payload T,
	cfg createConfig,
) (*T, error) {
	rb, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf(cfg.ErrMarshal, err)
	}

	req, err := http.NewRequestWithContext(ctx, httpMethodPost,
		fmt.Sprintf(cfg.PathFormat, cfg.HostURL, cfg.APIPath, cfg.Fields), bytes.NewReader(rb))
	if err != nil {
		return nil, fmt.Errorf(cfg.ErrCreate, err)
	}

	body, err := client.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf(cfg.ErrFetch, err)
	}

	var item T
	if err := json.Unmarshal(body, &item); err != nil {
		return nil, fmt.Errorf(cfg.ErrDecode, err)
	}

	return &item, nil
}

func updateAndDecode[T any](
	ctx context.Context,
	client *Client,
	id string,
	payload T,
	cfg updateConfig,
) (*T, error) {
	rb, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf(cfg.ErrMarshal, err)
	}

	req, err := http.NewRequestWithContext(ctx, httpMethodPost,
		fmt.Sprintf(cfg.PathFormat, cfg.HostURL, cfg.APIPath, id, cfg.Fields), bytes.NewReader(rb))
	if err != nil {
		return nil, fmt.Errorf(cfg.ErrCreate, err)
	}

	body, err := client.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf(cfg.ErrFetch, err)
	}

	var item T
	if err := json.Unmarshal(body, &item); err != nil {
		return nil, fmt.Errorf(cfg.ErrDecode, err)
	}

	return &item, nil
}

func deleteByID(
	ctx context.Context,
	client *Client,
	id string,
	cfg deleteConfig,
) error {
	req, err := http.NewRequestWithContext(ctx, httpMethodDelete,
		fmt.Sprintf("%s/%s/%s", cfg.HostURL, cfg.APIPath, id), nil)
	if err != nil {
		return fmt.Errorf(cfg.ErrCreate, err)
	}

	_, err = client.doRequest(req)
	if err != nil {
		if IsNotFoundError(err) {
			return nil
		}
		return fmt.Errorf(cfg.ErrFetch, err)
	}

	return nil
}
