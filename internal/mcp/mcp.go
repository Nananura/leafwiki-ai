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

	return s
}
