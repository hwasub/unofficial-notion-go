package notion

import (
	"encoding/json"
	"fmt"
	"sort"
)

// DirectSubpageLinks returns the child pages directly linked from the page
// identified by pageID within rawRecordMap. It collects both child-page and
// alias blocks and inline page mentions/links whose target is a direct child,
// de-duplicating by normalized page ID and skipping the root itself. It returns
// an error if the record map cannot be parsed or the root page is missing.
func DirectSubpageLinks(rawRecordMap json.RawMessage, pageID string) ([]SubpageLink, error) {
	var rm recordMap
	if err := json.Unmarshal(rawRecordMap, &rm); err != nil {
		return nil, err
	}
	rootID := NormalizeID(pageID)
	root, ok := rm.Block[rootID]
	if !ok {
		return nil, fmt.Errorf("notion root page %q not found", pageID)
	}

	seen := map[string]struct{}{}
	out := make([]SubpageLink, 0)
	appendLink := func(linkID string, title string) {
		if !IsNormalizedID(linkID) || linkID == rootID {
			return
		}
		if _, exists := seen[linkID]; exists {
			return
		}
		seen[linkID] = struct{}{}
		out = append(out, SubpageLink{PageID: linkID, Title: firstNonEmpty(title, "Page")})
	}
	walked := map[string]struct{}{}
	var walk func([]string)
	walk = func(ids []string) {
		for _, rawID := range ids {
			id := NormalizeID(rawID)
			blk, ok := rm.Block[id]
			if !ok {
				continue
			}
			// Guard against block-content cycles (A.content=[B], B.content=[A]),
			// which would otherwise drive walk into unbounded recursion.
			if _, done := walked[id]; done {
				continue
			}
			walked[id] = struct{}{}
			if blk.Type == "page" || blk.Type == "alias" {
				linkID, ok := linkedPageID(blk)
				if !ok || linkID == rootID {
					continue
				}
				appendLink(linkID, firstNonEmpty(plainText(blk.Properties["title"]), plainText(rm.Block[linkID].Properties["title"])))
				continue
			}
			for _, candidate := range richTextSubpageLinkCandidates(blk.Properties) {
				target, ok := rm.Block[candidate.PageID]
				if !ok || target.Type != "page" {
					continue
				}
				parentID := NormalizeID(target.ParentID)
				if parentID != "" && parentID != rootID {
					continue
				}
				appendLink(candidate.PageID, firstNonEmpty(plainText(target.Properties["title"]), candidate.Title))
			}
			walk(blk.Content)
		}
	}
	walk(root.Content)
	return out, nil
}

func richTextSubpageLinkCandidates(properties map[string]any) []subpageLinkCandidate {
	if len(properties) == 0 {
		return nil
	}
	keys := make([]string, 0, len(properties))
	for key := range properties {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]subpageLinkCandidate, 0)
	for _, key := range keys {
		out = append(out, richTextValueSubpageLinkCandidates(properties[key])...)
	}
	return out
}

func richTextValueSubpageLinkCandidates(value any) []subpageLinkCandidate {
	raw, ok := value.([]any)
	if !ok {
		return nil
	}
	out := make([]subpageLinkCandidate, 0)
	for _, part := range raw {
		arr, ok := part.([]any)
		if !ok || len(arr) < 2 {
			continue
		}
		label, _ := arr[0].(string)
		out = append(out, decorationSubpageLinkCandidates(arr[1], label)...)
	}
	return out
}

func decorationSubpageLinkCandidates(decorations any, label string) []subpageLinkCandidate {
	raw, ok := decorations.([]any)
	if !ok {
		return nil
	}
	out := make([]subpageLinkCandidate, 0)
	for _, deco := range raw {
		item, ok := deco.([]any)
		if !ok || len(item) == 0 {
			continue
		}
		kind := stringValue(item[0])
		switch kind {
		case "a", "lm":
			if len(item) > 1 {
				if pageID, ok := notionPageIDFromLink(stringValue(item[1])); ok {
					out = append(out, subpageLinkCandidate{PageID: pageID, Title: mentionRawTextLabel(label)})
				}
			}
		case "p":
			if len(item) > 1 {
				if pageID, ok := normalizedPageID(stringValue(item[1])); ok {
					out = append(out, subpageLinkCandidate{PageID: pageID, Title: mentionRawTextLabel(label)})
				}
			}
		case "‣":
			if len(item) > 1 {
				href, mentionLabel := externalLinkMentionValue(item[1])
				if pageID, ok := notionPageIDFromLink(href); ok {
					out = append(out, subpageLinkCandidate{PageID: pageID, Title: firstNonEmpty(mentionRawTextLabel(label), mentionRawTextLabel(mentionLabel))})
				}
			}
		}
	}
	return out
}
