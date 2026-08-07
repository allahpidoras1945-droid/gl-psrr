package domain

import "context"

type Scraper interface {
	Scrape(ctx context.Context, targetURL string) (*Lead, error)
}

type Extractor interface {
	ExtractContacts(rawContent string) ContactInfo
}

type Filter interface {
	IsCIS(lead *Lead) (bool, string)
	IsDuplicate(identifier string) bool
}

type TGValidator interface {
	ValidateUsername(ctx context.Context, username string) (*TGValidationResult, error)
	Close() error
}

type Exporter interface {
	Export(ctx context.Context, leads []*Lead, outputPath string) error
}

type Pipeline interface {
	Run(ctx context.Context, sources []string) ([]*Lead, error)
}
