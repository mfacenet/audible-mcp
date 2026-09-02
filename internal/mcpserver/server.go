package mcpserver

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/mfacenet/audible-mcp/internal/audible"
	"github.com/mfacenet/audible-mcp/internal/version"
)

// Options configure the Audible MCP server.
type Options struct {
	AuthFile string
	BaseURL  string
}

// New constructs the stdio MCP server with the Audible read tools and resources.
func New(opts Options) (*mcp.Server, error) {
	api, err := audible.FromAuthFile(audible.Options{
		AuthFile: opts.AuthFile,
		BaseURL:  opts.BaseURL,
	})
	if err != nil {
		return nil, err
	}

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "audible-mcp",
		Version: version.Version,
	}, &mcp.ServerOptions{
		Instructions: "Use the Audible tools to inspect the user's library, wishlist, collections, catalog metadata, and listening stats. Prefer read-only workflows.",
	})

	addTools(server, api, opts.AuthFile)
	addResources(server, api)
	return server, nil
}

type emptyInput struct{}

type pageInput struct {
	Page       *int `json:"page,omitempty" jsonschema:"1-based results page"`
	NumResults *int `json:"numResults,omitempty" jsonschema:"Number of items to return, up to 100"`
}

type asinInput struct {
	ASIN string `json:"asin" jsonschema:"Audible ASIN"`
}

type collectionInput struct {
	CollectionID string `json:"collectionId" jsonschema:"Collection id, such as __FAVORITES"`
}

type searchInput struct {
	Query             string `json:"query" jsonschema:"Text to match against titles and contributor names"`
	MaxPages          *int   `json:"maxPages,omitempty" jsonschema:"How many library pages to scan"`
	NumResultsPerPage *int   `json:"numResultsPerPage,omitempty" jsonschema:"Page size to fetch from the library endpoint"`
}

type inProgressInput struct {
	MaxPages          *int `json:"maxPages,omitempty" jsonschema:"How many library pages to scan"`
	NumResultsPerPage *int `json:"numResultsPerPage,omitempty" jsonschema:"Page size to fetch from the library endpoint"`
}

type statsInput struct {
	StartMonth *string `json:"startMonth,omitempty" jsonschema:"Start month in YYYY-MM format"`
	Months     *int    `json:"months,omitempty" jsonschema:"Number of monthly buckets to fetch"`
	Locale     *string `json:"locale,omitempty" jsonschema:"Audible locale, for example en_US"`
	Store      *string `json:"store,omitempty" jsonschema:"Stats store. Audible works for standard audiobook stats"`
}

func addTools(server *mcp.Server, api *audible.API, authFile string) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "audible_list_library",
		Description: "List titles from the authenticated Audible library.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in pageInput) (*mcp.CallToolResult, any, error) {
		return jsonResult(api.ListLibrary(ctx, audible.ListLibraryOptions{
			NumResults: derefInt(in.NumResults),
			Page:       derefInt(in.Page),
		}))
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "audible_get_library_item",
		Description: "Fetch a single library item by ASIN, including progress and download-related fields.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in asinInput) (*mcp.CallToolResult, any, error) {
		return jsonResult(api.GetLibraryItem(ctx, in.ASIN))
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "audible_list_wishlist",
		Description: "List titles in the authenticated Audible wishlist.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in pageInput) (*mcp.CallToolResult, any, error) {
		return jsonResult(api.ListWishlist(ctx, audible.ListWishlistOptions{
			NumResults: derefInt(in.NumResults),
			Page:       derefInt(in.Page),
		}))
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "audible_list_collections",
		Description: "List Audible collections for the authenticated account.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ emptyInput) (*mcp.CallToolResult, any, error) {
		return jsonResult(api.ListCollections(ctx))
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "audible_list_collection_items",
		Description: "List items inside an Audible collection by collection id.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in collectionInput) (*mcp.CallToolResult, any, error) {
		return jsonResult(api.ListCollectionItems(ctx, in.CollectionID))
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "audible_search_library",
		Description: "Search the authenticated library by title, ASIN, author, narrator, or series name using client-side filtering across the first few pages.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in searchInput) (*mcp.CallToolResult, any, error) {
		return jsonResult(api.SearchLibrary(ctx, audible.SearchLibraryOptions{
			Query:             in.Query,
			MaxPages:          derefInt(in.MaxPages),
			NumResultsPerPage: derefInt(in.NumResultsPerPage),
		}))
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "audible_list_in_progress_titles",
		Description: "List titles in the library that have started but are not yet finished.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in inProgressInput) (*mcp.CallToolResult, any, error) {
		return jsonResult(api.ListInProgressTitles(ctx, audible.InProgressOptions{
			MaxPages:          derefInt(in.MaxPages),
			NumResultsPerPage: derefInt(in.NumResultsPerPage),
		}))
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "audible_get_content_metadata",
		Description: "Fetch content metadata for an Audible ASIN, including chapter information.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in asinInput) (*mcp.CallToolResult, any, error) {
		return jsonResult(api.GetContentMetadata(ctx, in.ASIN))
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "audible_get_chapters",
		Description: "Fetch chapter information for an Audible ASIN.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in asinInput) (*mcp.CallToolResult, any, error) {
		return jsonResult(api.GetChapters(ctx, in.ASIN))
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "audible_get_catalog_product",
		Description: "Fetch catalog metadata for an Audible ASIN from the catalog products endpoint.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in asinInput) (*mcp.CallToolResult, any, error) {
		return jsonResult(api.GetCatalogProduct(ctx, in.ASIN))
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "audible_get_listening_stats",
		Description: "Fetch aggregate listening stats. The default window is the current month plus the prior two months.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in statsInput) (*mcp.CallToolResult, any, error) {
		return jsonResult(api.GetListeningStats(ctx, audible.ListeningStatsOptions{
			Locale:     derefString(in.Locale),
			Months:     derefInt(in.Months),
			StartMonth: derefString(in.StartMonth),
			Store:      derefString(in.Store),
		}))
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "audible_get_auth_status",
		Description: "Return local auth metadata for the configured Audible device registration.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, _ emptyInput) (*mcp.CallToolResult, any, error) {
		status := api.AuthStatus()
		status["authFile"] = authFile
		return jsonValue(status)
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "audible_validate_auth",
		Description: "Validate signed-auth access by performing a lightweight library read and refreshing tokens if needed.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ emptyInput) (*mcp.CallToolResult, any, error) {
		return jsonResult(api.ValidateAuth(ctx))
	})
}

func addResources(server *mcp.Server, api *audible.API) {
	server.AddResource(&mcp.Resource{
		Name:        "audible-auth-status",
		URI:         "audible://auth/status",
		Description: "Current signed-auth device metadata for the configured Audible auth file.",
		MIMEType:    "application/json",
	}, func(_ context.Context, _ *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		status := api.AuthStatus()
		delete(status, "authFile")
		delete(status, "websiteCookieNames")
		return jsonResource("audible://auth/status", status)
	})

	server.AddResource(&mcp.Resource{
		Name:        "audible-wishlist",
		URI:         "audible://wishlist",
		Description: "Current Audible wishlist.",
		MIMEType:    "application/json",
	}, func(ctx context.Context, _ *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		data, err := api.ListWishlist(ctx, audible.ListWishlistOptions{})
		if err != nil {
			return nil, err
		}
		return jsonResource("audible://wishlist", data)
	})

	server.AddResource(&mcp.Resource{
		Name:        "audible-collections",
		URI:         "audible://collections",
		Description: "Current Audible collections list.",
		MIMEType:    "application/json",
	}, func(ctx context.Context, _ *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		data, err := api.ListCollections(ctx)
		if err != nil {
			return nil, err
		}
		return jsonResource("audible://collections", data)
	})

	server.AddResourceTemplate(&mcp.ResourceTemplate{
		Name:        "audible-library-item",
		URITemplate: "audible://library/{asin}",
		Description: "Read a library item by ASIN.",
		MIMEType:    "application/json",
	}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		asin := firstPathSegment(req.Params.URI)
		data, err := api.GetLibraryItem(ctx, asin)
		if err != nil {
			return nil, err
		}
		return jsonResource("audible://library/"+asin, data)
	})

	server.AddResourceTemplate(&mcp.ResourceTemplate{
		Name:        "audible-collection-items",
		URITemplate: "audible://collections/{collectionId}/items",
		Description: "Read items from a collection by collection id.",
		MIMEType:    "application/json",
	}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		id := firstPathSegment(req.Params.URI)
		data, err := api.ListCollectionItems(ctx, id)
		if err != nil {
			return nil, err
		}
		return jsonResource("audible://collections/"+id+"/items", data)
	})

	server.AddResourceTemplate(&mcp.ResourceTemplate{
		Name:        "audible-content-metadata",
		URITemplate: "audible://content/{asin}/metadata",
		Description: "Read chapter and content metadata by ASIN.",
		MIMEType:    "application/json",
	}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		asin := firstPathSegment(req.Params.URI)
		data, err := api.GetContentMetadata(ctx, asin)
		if err != nil {
			return nil, err
		}
		return jsonResource("audible://content/"+asin+"/metadata", data)
	})

	server.AddResourceTemplate(&mcp.ResourceTemplate{
		Name:        "audible-catalog-product",
		URITemplate: "audible://catalog/{asin}",
		Description: "Read catalog product metadata by ASIN.",
		MIMEType:    "application/json",
	}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		asin := firstPathSegment(req.Params.URI)
		data, err := api.GetCatalogProduct(ctx, asin)
		if err != nil {
			return nil, err
		}
		return jsonResource("audible://catalog/"+asin, data)
	})
}

func jsonResult[T any](v T, err error) (*mcp.CallToolResult, any, error) {
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
			IsError: true,
		}, nil, nil
	}
	return jsonValue(v)
}

func jsonValue(v any) (*mcp.CallToolResult, any, error) {
	raw, err := marshalPretty(v)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
			IsError: true,
		}, nil, nil
	}
	var structured any
	if err := json.Unmarshal(raw, &structured); err != nil {
		structured = v
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(raw)}},
	}, structured, nil
}

func jsonResource(uri string, v any) (*mcp.ReadResourceResult, error) {
	raw, err := marshalPretty(v)
	if err != nil {
		return nil, err
	}
	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{
			{
				URI:      uri,
				MIMEType: "application/json",
				Text:     string(raw),
			},
		},
	}, nil
}

func marshalPretty(v any) ([]byte, error) {
	switch t := v.(type) {
	case json.RawMessage:
		var decoded any
		if err := json.Unmarshal(t, &decoded); err != nil {
			return json.MarshalIndent(t, "", "  ")
		}
		return json.MarshalIndent(decoded, "", "  ")
	default:
		return json.MarshalIndent(v, "", "  ")
	}
}

func derefInt(v *int) int {
	if v == nil {
		return 0
	}
	return *v
}

func derefString(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func firstPathSegment(rawURI string) string {
	u, err := url.Parse(rawURI)
	if err != nil {
		return ""
	}
	path := strings.Trim(u.Path, "/")
	seg, _, _ := strings.Cut(path, "/")
	return seg
}
