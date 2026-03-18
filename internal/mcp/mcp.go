package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/perber/wiki/internal/core/tree"
	"github.com/perber/wiki/internal/wiki"
)

const mcpUserID = "system_mcp"

func SetupMCPServer(w *wiki.Wiki) *server.MCPServer {
	s := server.NewMCPServer("leafwiki", "1.0.0")

	// 1. Tool: read_page
	readPageTool := mcp.NewTool("read_page",
		mcp.WithDescription("Read the markdown content of a page by its path"),
		mcp.WithString("path", mcp.Required(), mcp.Description("The path of the page (e.g., /home, /guide/setup)")),
	)

	s.AddTool(readPageTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		pathParam := req.GetString("path", "")
		if pathParam == "" {
			return mcp.NewToolResultError("path is required"), nil
		}

		// Normalize: strip leading/trailing slashes since tree uses slug segments
		pathParam = strings.Trim(pathParam, "/")
		
		page, err := w.FindByPath(pathParam)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Page not found: %s", pathParam)), nil
		}
		
		return mcp.NewToolResultText(page.Content), nil
	})

	// 2. Tool: search_wiki
	searchWikiTool := mcp.NewTool("search_wiki",
		mcp.WithDescription("Search the wiki for pages containing a keyword or matching a query"),
		mcp.WithString("query", mcp.Required(), mcp.Description("The search query")),
	)

	s.AddTool(searchWikiTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query := req.GetString("query", "")
		if query == "" {
			return mcp.NewToolResultError("query is required"), nil
		}
		
		results, err := w.Search(query, 0, 10) // offset 0, limit 10
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Search error: %v", err)), nil
		}
		
		if len(results.Items) == 0 {
			return mcp.NewToolResultText("No results found."), nil
		}

		var b strings.Builder
		for _, item := range results.Items {
			b.WriteString(fmt.Sprintf("- [%s] %s\n  Snippet: %s\n\n", item.Path, item.Title, item.Excerpt))
		}
		
		return mcp.NewToolResultText(b.String()), nil
	})

	// 3. Tool: list_pages
	listPagesTool := mcp.NewTool("list_pages",
		mcp.WithDescription("List all pages in the wiki as a tree structure"),
	)

	s.AddTool(listPagesTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		rootNode := w.GetTree()
		if rootNode == nil {
			return mcp.NewToolResultText("Wiki is empty."), nil
		}

		var b strings.Builder
		var walk func(node *tree.PageNode, indent string)
		walk = func(node *tree.PageNode, indent string) {
			if node == nil {
				return
			}
			if node.ID != "root" {
				kindLabel := "page"
				if node.Kind == tree.NodeKindSection {
					kindLabel = "section"
				}
				b.WriteString(fmt.Sprintf("%s- %s (path: %s, kind: %s)\n", indent, node.Title, node.CalculatePath(), kindLabel))
			}
			for _, child := range node.Children {
				if node.ID == "root" {
					walk(child, indent)
				} else {
					walk(child, indent+"  ")
				}
			}
		}
		
		walk(rootNode, "")
		text := b.String()
		if text == "" {
			return mcp.NewToolResultText("Wiki is empty."), nil
		}
		return mcp.NewToolResultText(text), nil
	})

	// 4. Tool: create_page
	createPageTool := mcp.NewTool("create_page",
		mcp.WithDescription("Create a new page or section at the given path. Intermediate sections are created automatically. If the page already exists at the path, its current content is returned instead."),
		mcp.WithString("path", mcp.Required(), mcp.Description("The full path for the page (e.g., guides/getting-started). Each segment becomes a slug.")),
		mcp.WithString("title", mcp.Required(), mcp.Description("The display title for the page")),
		mcp.WithString("content", mcp.Description("Optional initial markdown content for the page")),
		mcp.WithString("kind", mcp.Description("The kind of node to create: 'page' (default) or 'section'")),
	)

	s.AddTool(createPageTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		pathParam := strings.Trim(req.GetString("path", ""), "/")
		title := req.GetString("title", "")
		content := req.GetString("content", "")
		kindStr := req.GetString("kind", "page")

		if pathParam == "" {
			return mcp.NewToolResultError("path is required"), nil
		}
		if title == "" {
			return mcp.NewToolResultError("title is required"), nil
		}

		kind := tree.NodeKindPage
		if kindStr == "section" {
			kind = tree.NodeKindSection
		}

		page, err := w.EnsurePath(mcpUserID, pathParam, title, &kind)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to create page: %v", err)), nil
		}

		// If content is provided, update the page with it
		if content != "" {
			_, err = w.UpdatePage(mcpUserID, page.ID, page.Title, page.Slug, &content, nil)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Page created but failed to set content: %v", err)), nil
			}
		}

		return mcp.NewToolResultText(fmt.Sprintf("Page created successfully.\nID: %s\nPath: %s\nTitle: %s\nKind: %s", page.ID, page.CalculatePath(), page.Title, page.Kind)), nil
	})

	// 5. Tool: update_page
	updatePageTool := mcp.NewTool("update_page",
		mcp.WithDescription("Update an existing page's content and/or title by its path"),
		mcp.WithString("path", mcp.Required(), mcp.Description("The path of the page to update (e.g., /guides/getting-started)")),
		mcp.WithString("content", mcp.Description("New markdown content for the page")),
		mcp.WithString("title", mcp.Description("New title for the page (if not provided, keeps existing title)")),
	)

	s.AddTool(updatePageTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		pathParam := strings.Trim(req.GetString("path", ""), "/")
		content := req.GetString("content", "")
		newTitle := req.GetString("title", "")

		if pathParam == "" {
			return mcp.NewToolResultError("path is required"), nil
		}

		// Find existing page
		page, err := w.FindByPath(pathParam)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Page not found: %s", pathParam)), nil
		}

		title := page.Title
		if newTitle != "" {
			title = newTitle
		}

		var contentPtr *string
		if content != "" {
			contentPtr = &content
		}

		updated, err := w.UpdatePage(mcpUserID, page.ID, title, page.Slug, contentPtr, nil)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to update page: %v", err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("Page updated successfully.\nID: %s\nPath: %s\nTitle: %s", updated.ID, updated.CalculatePath(), updated.Title)), nil
	})

	// 6. Tool: delete_page
	deletePageTool := mcp.NewTool("delete_page",
		mcp.WithDescription("Delete a page or section by its path. Use recursive=true to delete a section with all its children."),
		mcp.WithString("path", mcp.Required(), mcp.Description("The path of the page to delete")),
		mcp.WithBoolean("recursive", mcp.Description("If true, delete the section and all its children. Required for non-empty sections. Default: false")),
	)

	s.AddTool(deletePageTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		pathParam := strings.Trim(req.GetString("path", ""), "/")
		recursive := req.GetBool("recursive", false)

		if pathParam == "" {
			return mcp.NewToolResultError("path is required"), nil
		}

		page, err := w.FindByPath(pathParam)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Page not found: %s", pathParam)), nil
		}

		err = w.DeletePage(mcpUserID, page.ID, recursive)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to delete page: %v", err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("Page deleted successfully: %s", pathParam)), nil
	})

	// 7. Tool: move_page
	movePageTool := mcp.NewTool("move_page",
		mcp.WithDescription("Move a page or section to a new parent location in the wiki tree. The page keeps its slug and content but changes its parent."),
		mcp.WithString("path", mcp.Required(), mcp.Description("The current path of the page/section to move (e.g., guides/old-location/my-page)")),
		mcp.WithString("new_parent_path", mcp.Description("The path of the new parent section. Use empty string or 'root' to move to the top level.")),
	)

	s.AddTool(movePageTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		pathParam := strings.Trim(req.GetString("path", ""), "/")
		newParentPath := strings.Trim(req.GetString("new_parent_path", ""), "/")

		if pathParam == "" {
			return mcp.NewToolResultError("path is required"), nil
		}

		// Find the page to move
		page, err := w.FindByPath(pathParam)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Page not found: %s", pathParam)), nil
		}

		// Resolve the new parent ID
		newParentID := "root"
		if newParentPath != "" && newParentPath != "root" {
			parentPage, err := w.FindByPath(newParentPath)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("New parent not found: %s", newParentPath)), nil
			}
			newParentID = parentPage.ID
		}

		err = w.MovePage(mcpUserID, page.ID, newParentID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to move page: %v", err)), nil
		}

		// Re-fetch to get updated path
		movedPage, err := w.GetPage(page.ID)
		if err != nil {
			return mcp.NewToolResultText(fmt.Sprintf("Page moved successfully (could not fetch updated path: %v)", err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("Page moved successfully.\nID: %s\nNew path: %s\nTitle: %s", movedPage.ID, movedPage.CalculatePath(), movedPage.Title)), nil
	})

	// 8. Tool: edit_page
	editPageTool := mcp.NewTool("edit_page",
		mcp.WithDescription("Search & replace content within a page. Good for targeted edits without rewriting the entire file."),
		mcp.WithString("path", mcp.Required(), mcp.Description("The path of the page to edit")),
		mcp.WithString("target", mcp.Required(), mcp.Description("The exact string to replace")),
		mcp.WithString("replacement", mcp.Required(), mcp.Description("The content to replace it with")),
	)

	s.AddTool(editPageTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		pathParam := strings.Trim(req.GetString("path", ""), "/")
		target := req.GetString("target", "")
		replacement := req.GetString("replacement", "")

		if pathParam == "" || target == "" {
			return mcp.NewToolResultError("path and target are required"), nil
		}

		page, err := w.FindByPath(pathParam)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Page not found: %s", pathParam)), nil
		}

		if !strings.Contains(page.Content, target) {
			return mcp.NewToolResultError("Target string not found in page content"), nil
		}

		newContent := strings.Replace(page.Content, target, replacement, 1)

		updated, err := w.UpdatePage(mcpUserID, page.ID, page.Title, page.Slug, &newContent, nil)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to update page: %v", err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("Page edited successfully.\nID: %s\nPath: %s\nTarget replaced.", updated.ID, updated.CalculatePath())), nil
	})

	// 9. Tool: get_page_info
	getPageInfoTool := mcp.NewTool("get_page_info",
		mcp.WithDescription("Get metadata (ID, path, kind, dates) for a page without loading its full content"),
		mcp.WithString("path", mcp.Required(), mcp.Description("The path of the page")),
	)

	s.AddTool(getPageInfoTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		pathParam := strings.Trim(req.GetString("path", ""), "/")
		if pathParam == "" {
			return mcp.NewToolResultError("path is required"), nil
		}

		page, err := w.FindByPath(pathParam)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Page not found: %s", pathParam)), nil
		}

		kindLabel := "page"
		if page.Kind == tree.NodeKindSection {
			kindLabel = "section"
		}

		info := fmt.Sprintf("ID: %s\nTitle: %s\nPath: %s\nKind: %s\nCreated: %s\nUpdated: %s\nCreator ID: %s",
			page.ID, page.Title, page.CalculatePath(), kindLabel, page.Metadata.CreatedAt.Format("2006-01-02 15:04:05"), page.Metadata.UpdatedAt.Format("2006-01-02 15:04:05"), page.Metadata.CreatorID)
		
		return mcp.NewToolResultText(info), nil
	})

	// 10. Tool: get_backlinks
	getBacklinksTool := mcp.NewTool("get_backlinks",
		mcp.WithDescription("Get all pages linking TO a given page"),
		mcp.WithString("path", mcp.Required(), mcp.Description("The path of the page")),
	)

	s.AddTool(getBacklinksTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		pathParam := strings.Trim(req.GetString("path", ""), "/")
		if pathParam == "" {
			return mcp.NewToolResultError("path is required"), nil
		}

		page, err := w.FindByPath(pathParam)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Page not found: %s", pathParam)), nil
		}

		res, err := w.GetBacklinks(page.ID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Error getting backlinks: %v", err)), nil
		}

		if len(res.Backlinks) == 0 {
			return mcp.NewToolResultText("No backlinks found."), nil
		}

		var b strings.Builder
		for _, link := range res.Backlinks {
			b.WriteString(fmt.Sprintf("- [%s] (ID: %s)\n", link.FromTitle, link.FromPageID))
		}
		
		return mcp.NewToolResultText(b.String()), nil
	})

	// Resources
	// Resource: wiki://pages/{path}
	s.AddResourceTemplate(
		mcp.NewResourceTemplate("wiki://pages/{path}", "Wiki Page",
			mcp.WithTemplateDescription("Access the markdown content of any Wiki page by its path"),
			mcp.WithTemplateMIMEType("text/markdown"),
		),
		func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			path := strings.Trim(strings.TrimPrefix(req.Params.URI, "wiki://pages/"), "/")
			page, err := w.FindByPath(path)
			if err != nil {
				return nil, fmt.Errorf("page not found: %s", path)
			}
			return []mcp.ResourceContents{
				mcp.TextResourceContents{
					URI:      req.Params.URI,
					Text:     page.Content,
					MIMEType: "text/markdown",
				},
			}, nil
		},
	)

	// Resource: wiki://tree
	s.AddResource(
		mcp.Resource{
			URI:         "wiki://tree",
			Name:        "Wiki Tree",
			Description: "Full hierarchical tree structure of the entire wiki",
			MIMEType:    "text/plain",
		},
		func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			rootNode := w.GetTree()
			if rootNode == nil {
				return nil, fmt.Errorf("wiki is empty")
			}
			var b strings.Builder
			var walk func(node *tree.PageNode, indent string)
			walk = func(node *tree.PageNode, indent string) {
				if node == nil {
					return
				}
				if node.ID != "root" {
					kindLabel := "page"
					if node.Kind == tree.NodeKindSection {
						kindLabel = "section"
					}
					b.WriteString(fmt.Sprintf("%s- %s (path: %s, kind: %s)\n", indent, node.Title, node.CalculatePath(), kindLabel))
				}
				for _, child := range node.Children {
					if node.ID == "root" {
						walk(child, indent)
					} else {
						walk(child, indent+"  ")
					}
				}
			}
			walk(rootNode, "")
			return []mcp.ResourceContents{
				mcp.TextResourceContents{
					URI:      "wiki://tree",
					Text:     b.String(),
					MIMEType: "text/plain",
				},
			}, nil
		},
	)

	// Prompts
	s.AddPrompt(mcp.Prompt{
		Name:        "summarize-page",
		Description: "Read a page and generate a concise summary",
		Arguments: []mcp.PromptArgument{
			{Name: "path", Description: "Path of the page to summarize", Required: true},
			{Name: "style", Description: "Summary style: 'prose' (default), 'bullet', or 'tldr'", Required: false},
		},
	}, func(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		pathParam := req.Params.Arguments["path"]
		style := req.Params.Arguments["style"]
		if style == "" {
			style = "prose"
		}

		var styleInstruction string
		switch style {
		case "bullet":
			styleInstruction = "Write the summary as a concise bullet-point list of the key points."
		case "tldr":
			styleInstruction = "Write a single short paragraph (2-3 sentences) TL;DR summary."
		default:
			styleInstruction = "Write a short prose summary of 1-2 paragraphs covering the main topics."
		}

		return &mcp.GetPromptResult{
			Messages: []mcp.PromptMessage{
				{
					Role: mcp.RoleUser,
					Content: mcp.TextContent{
						Type: "text",
						Text: fmt.Sprintf("You are a helpful wiki assistant. Use the `read_page` tool to fetch the page content first. If the page is not found or returns an error, inform the user clearly instead of guessing the content.\n\nPlease read the wiki page at `%s` and summarize its contents.\n\n%s", pathParam, styleInstruction),
					},
				},
			},
		}, nil
	})

	s.AddPrompt(mcp.Prompt{
		Name:        "document-from-notes",
		Description: "Take rough notes and create a well-structured wiki page",
		Arguments: []mcp.PromptArgument{
			{Name: "path", Description: "Path where the new page should be created (e.g. guides/setup)", Required: true},
			{Name: "title", Description: "Display title for the page", Required: true},
			{Name: "notes", Description: "The rough notes to convert into documentation", Required: true},
		},
	}, func(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		pathParam := req.Params.Arguments["path"]
		titleParam := req.Params.Arguments["title"]
		notesParam := req.Params.Arguments["notes"]
		return &mcp.GetPromptResult{
			Messages: []mcp.PromptMessage{
				{
					Role: mcp.RoleUser,
					Content: mcp.TextContent{
						Type: "text",
						Text: fmt.Sprintf("You are a technical documentation assistant. Follow these steps:\n1. Use `get_page_info` to check whether the target path already exists.\n2. Organize the notes below into a well-structured Markdown document using ATX-style headings (##, ###), fenced code blocks with language tags, bullet or numbered lists where appropriate, and no YAML frontmatter.\n3. If the page does not exist, use `create_page` with the provided title and content. If it already exists, use `update_page` to replace the content rather than creating a duplicate.\n\nPath: `%s`\nTitle: %s\n\nNotes:\n%s", pathParam, titleParam, notesParam),
					},
				},
			},
		}, nil
	})

	// 3. Prompt: fix-broken-links
	s.AddPrompt(mcp.Prompt{
		Name:        "fix-broken-links",
		Description: "Scan the wiki for broken internal links and offer to repair them",
	}, func(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		return &mcp.GetPromptResult{
			Messages: []mcp.PromptMessage{
				{
					Role: mcp.RoleUser,
					Content: mcp.TextContent{
						Type: "text",
						Text: "You are a wiki maintenance assistant. Scan the entire wiki for broken internal links and fix any you can resolve confidently.\n\nFollow these steps:\n1. Use `list_pages` to get every page in the wiki.\n2. For each page, use `get_backlinks` to identify any links marked as broken.\n3. For each broken link found, use `read_page` on the source page to inspect the surrounding context.\n4. Use `search_wiki` to find the likely correct target page.\n5. Use `edit_page` to replace each broken link reference with the correct path.\n6. Report a summary of all links fixed and any that could not be resolved automatically.",
					},
				},
			},
		}, nil
	})

	// 4. Prompt: review-page
	s.AddPrompt(mcp.Prompt{
		Name:        "review-page",
		Description: "Review a page for clarity, completeness, and quality, with optional inline fixes",
		Arguments: []mcp.PromptArgument{
			{Name: "path", Description: "Path of the page to review", Required: true},
			{Name: "fix", Description: "If 'true', apply suggested improvements directly using edit_page. Default: false (report only).", Required: false},
		},
	}, func(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		pathParam := req.Params.Arguments["path"]
		fix := req.Params.Arguments["fix"]
		actionInstruction := "Report your findings as a structured list — do not modify the page."
		if fix == "true" {
			actionInstruction = "For each issue you are confident about, apply the fix directly using `edit_page`. List every change made."
		}
		return &mcp.GetPromptResult{
			Messages: []mcp.PromptMessage{
				{
					Role: mcp.RoleUser,
					Content: mcp.TextContent{
						Type: "text",
						Text: fmt.Sprintf("You are a technical writing reviewer. Use `read_page` to fetch the page at `%s`, then evaluate it for: clarity and conciseness, logical structure and heading hierarchy, missing or incomplete sections, outdated or ambiguous phrasing, and broken or suspicious link patterns. Be specific and actionable.\n\n%s", pathParam, actionInstruction),
					},
				},
			},
		}, nil
	})

	// 5. Prompt: expand-stub
	s.AddPrompt(mcp.Prompt{
		Name:        "expand-stub",
		Description: "Detect a short stub page and expand it with additional content",
		Arguments: []mcp.PromptArgument{
			{Name: "path", Description: "Path of the stub page to expand", Required: true},
			{Name: "instructions", Description: "What to add or expand on (e.g. 'add prerequisites and a quickstart example')", Required: true},
		},
	}, func(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		pathParam := req.Params.Arguments["path"]
		instructions := req.Params.Arguments["instructions"]
		return &mcp.GetPromptResult{
			Messages: []mcp.PromptMessage{
				{
					Role: mcp.RoleUser,
					Content: mcp.TextContent{
						Type: "text",
						Text: fmt.Sprintf("You are a technical documentation writer. Expand the stub page at `%s` as follows:\n1. Use `read_page` to fetch the current content.\n2. Write the expanded version, preserving all existing content and appending or integrating new sections as instructed.\n3. Use ATX headings (##, ###), fenced code blocks with language tags, and no YAML frontmatter.\n4. Use `update_page` to save the result.\n\nWhat to add: %s", pathParam, instructions),
					},
				},
			},
		}, nil
	})

	// 6. Prompt: reorganize-section
	s.AddPrompt(mcp.Prompt{
		Name:        "reorganize-section",
		Description: "Propose and execute a better hierarchy for the children of a wiki section",
		Arguments: []mcp.PromptArgument{
			{Name: "section_path", Description: "Path of the section to reorganize (e.g. guides)", Required: true},
		},
	}, func(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		sectionPath := req.Params.Arguments["section_path"]
		return &mcp.GetPromptResult{
			Messages: []mcp.PromptMessage{
				{
					Role: mcp.RoleUser,
					Content: mcp.TextContent{
						Type: "text",
						Text: fmt.Sprintf("You are a wiki information architect. Analyse the section at `%s` and propose a better organisation for its child pages.\n\n1. Use `list_pages` to see the full wiki tree.\n2. Use `get_page_info` and `read_page` on the section's children to understand their content.\n3. Propose a clearer hierarchy with a brief rationale for each move.\n4. Show the proposed structure and wait for confirmation before making any changes.\n5. Execute approved moves using `move_page`.", sectionPath),
					},
				},
			},
		}, nil
	})

	// 7. Prompt: generate-index-page
	s.AddPrompt(mcp.Prompt{
		Name:        "generate-index-page",
		Description: "Generate a structured index/overview page for a section from its child pages",
		Arguments: []mcp.PromptArgument{
			{Name: "section_path", Description: "Path of the section to index (e.g. guides)", Required: true},
			{Name: "index_path", Description: "Path where the index page should be saved (defaults to section_path/index if omitted)", Required: false},
		},
	}, func(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		sectionPath := req.Params.Arguments["section_path"]
		indexPath := req.Params.Arguments["index_path"]
		if indexPath == "" {
			indexPath = sectionPath + "/index"
		}
		return &mcp.GetPromptResult{
			Messages: []mcp.PromptMessage{
				{
					Role: mcp.RoleUser,
					Content: mcp.TextContent{
						Type: "text",
						Text: fmt.Sprintf("You are a documentation assistant. Generate an index page for the `%s` section and save it at `%s`.\n\n1. Use `list_pages` to find all child pages of the section.\n2. Use `read_page` on each child to extract a one-sentence description of its content.\n3. Write a Markdown index page with: a short introductory paragraph, a table or bullet list of child pages with their path and description, and a 'See also' section for related sections found via `search_wiki`.\n4. Use `get_page_info` to check if the index path already exists, then use `create_page` or `update_page` accordingly.", sectionPath, indexPath),
					},
				},
			},
		}, nil
	})

	// 8. Prompt: find-related-pages
	s.AddPrompt(mcp.Prompt{
		Name:        "find-related-pages",
		Description: "Search the wiki for all pages related to a topic using multiple query variations",
		Arguments: []mcp.PromptArgument{
			{Name: "topic", Description: "The topic or concept to search for", Required: true},
		},
	}, func(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		topic := req.Params.Arguments["topic"]
		return &mcp.GetPromptResult{
			Messages: []mcp.PromptMessage{
				{
					Role: mcp.RoleUser,
					Content: mcp.TextContent{
						Type: "text",
						Text: fmt.Sprintf("You are a wiki research assistant. Find all pages related to the topic: **%s**\n\n1. Identify 3-5 query variations or synonyms for the topic.\n2. Run `search_wiki` for each variation.\n3. Deduplicate the results across all queries.\n4. Return a ranked list grouped by relevance, showing the page path, title, and excerpt for each result.", topic),
					},
				},
			},
		}, nil
	})

	// 9. Prompt: audit-orphan-pages
	s.AddPrompt(mcp.Prompt{
		Name:        "audit-orphan-pages",
		Description: "Find pages with no inbound links that may need to be linked or removed",
	}, func(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		return &mcp.GetPromptResult{
			Messages: []mcp.PromptMessage{
				{
					Role: mcp.RoleUser,
					Content: mcp.TextContent{
						Type: "text",
						Text: "You are a wiki maintenance assistant. Scan the entire wiki and produce a report of all pages that have no inbound links, then suggest linking opportunities for each.\n\n1. Use `list_pages` to get every page in the wiki.\n2. For each page, call `get_backlinks` and check if the backlink count is zero.\n3. Compile a list of all orphaned pages (path, title, kind).\n4. For each orphan, use `search_wiki` with the page title to suggest existing pages that could link to it.\n5. Present the full orphan report with suggested linking opportunities.",
					},
				},
			},
		}, nil
	})

	// 10. Prompt: translate-page
	s.AddPrompt(mcp.Prompt{
		Name:        "translate-page",
		Description: "Translate a wiki page to another language and save it at a parallel path",
		Arguments: []mcp.PromptArgument{
			{Name: "path", Description: "Path of the page to translate (e.g. guides/setup)", Required: true},
			{Name: "language", Description: "Target language (e.g. 'French', 'German', 'Spanish')", Required: true},
			{Name: "language_code", Description: "Short language code used as path prefix (e.g. 'fr', 'de', 'es')", Required: true},
		},
	}, func(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		pathParam := req.Params.Arguments["path"]
		language := req.Params.Arguments["language"]
		langCode := req.Params.Arguments["language_code"]
		targetPath := langCode + "/" + pathParam
		return &mcp.GetPromptResult{
			Messages: []mcp.PromptMessage{
				{
					Role: mcp.RoleUser,
					Content: mcp.TextContent{
						Type: "text",
						Text: fmt.Sprintf("You are a professional technical translator. Translate the wiki page at `%s` into %s and save the result at `%s`.\n\n1. Use `read_page` to fetch the original content.\n2. Translate all Markdown content — headings, prose, and list items — preserving Markdown formatting exactly. Do not translate code blocks or internal link paths.\n3. Use `get_page_info` to check if the target path already exists, then use `create_page` or `update_page` accordingly.", pathParam, language, targetPath),
					},
				},
			},
		}, nil
	})

	// 11. Prompt: changelog-entry
	s.AddPrompt(mcp.Prompt{
		Name:        "changelog-entry",
		Description: "Append a formatted changelog entry to a designated changelog page",
		Arguments: []mcp.PromptArgument{
			{Name: "changelog_path", Description: "Path of the changelog page (e.g. project/changelog)", Required: true},
			{Name: "version", Description: "Version or date label for the entry (e.g. 'v1.2.0' or '2026-03-17')", Required: true},
			{Name: "changes", Description: "Description of the changes to record", Required: true},
		},
	}, func(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		changelogPath := req.Params.Arguments["changelog_path"]
		version := req.Params.Arguments["version"]
		changes := req.Params.Arguments["changes"]
		return &mcp.GetPromptResult{
			Messages: []mcp.PromptMessage{
				{
					Role: mcp.RoleUser,
					Content: mcp.TextContent{
						Type: "text",
						Text: fmt.Sprintf("You are a release documentation assistant. Add a new changelog entry to `%s`.\n\n1. Use `read_page` to fetch the current changelog content.\n2. Format the new entry as a level-2 heading (`## <version>`) followed by a bullet list of changes.\n3. Insert the new entry directly after the first level-1 heading (or at the top if none exists), so newest entries appear first.\n4. Use `edit_page` with the exact surrounding text as the target to insert the entry without altering any existing content.\n\nVersion: %s\nChanges:\n%s", changelogPath, version, changes),
					},
				},
			},
		}, nil
	})

	return s
}
