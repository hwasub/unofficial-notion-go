package notion

import (
	"fmt"
	"html"
	"sort"
	"strings"
)

func renderTable(out *strings.Builder, rm recordMap, blk block, input RenderInput) {
	rows := tableRowBlocks(rm, blk)
	columns := tableColumnOrder(blk)
	if len(columns) == 0 && len(rows) > 0 {
		columns = tableRowColumns(rows[0])
	}
	columnFormats := tableColumnFormats(blk)
	resolver := notionMentionResolver(rm, input)
	out.WriteString(`<div class="notion-table-wrap"><table class="notion-simple-table">`)
	if len(columns) > 0 {
		out.WriteString(`<colgroup>`)
		for _, column := range columns {
			out.WriteString(`<col`)
			if style := notionTableColumnStyle(columnFormats[column]); style != "" {
				out.WriteString(` style="`)
				out.WriteString(style)
				out.WriteString(`"`)
			}
			out.WriteString(`>`)
		}
		out.WriteString(`</colgroup>`)
	}
	out.WriteString(`<tbody>`)
	for rowIndex, row := range rows {
		rowColumns := columns
		if len(rowColumns) == 0 {
			rowColumns = tableRowColumns(row)
		}
		out.WriteString(`<tr`)
		if className := notionBlockColorClass(row); className != "" {
			out.WriteString(` class="`)
			out.WriteString(html.EscapeString(className))
			out.WriteString(`"`)
		}
		out.WriteString(`>`)
		for columnIndex, column := range rowColumns {
			cellTag := "td"
			scope := ""
			if nativeTableHeaderCell(blk, rowIndex, columnIndex) {
				cellTag = "th"
				if rowIndex == 0 {
					scope = "col"
				} else if columnIndex == 0 {
					scope = "row"
				}
			}
			out.WriteString(`<`)
			out.WriteString(cellTag)
			if scope != "" {
				out.WriteString(` scope="`)
				out.WriteString(scope)
				out.WriteString(`"`)
			}
			if className := notionTableCellColorClass(columnFormats[column]); className != "" {
				out.WriteString(` class="`)
				out.WriteString(html.EscapeString(className))
				out.WriteString(`"`)
			}
			out.WriteString(`>`)
			if cell := richTextWithResolver(row.Properties[column], resolver); cell != "" {
				out.WriteString(cell)
			} else {
				out.WriteString(`&nbsp;`)
			}
			out.WriteString(`</`)
			out.WriteString(cellTag)
			out.WriteString(`>`)
		}
		out.WriteString(`</tr>`)
	}
	out.WriteString(`</tbody></table></div>`)
}

func tableRowBlocks(rm recordMap, blk block) []block {
	rows := make([]block, 0, len(blk.Content))
	for _, child := range blk.Content {
		row, ok := rm.Block[NormalizeID(child)]
		if ok && row.Type == "table_row" {
			rows = append(rows, row)
		}
	}
	return rows
}

func nativeTableHeaderCell(blk block, rowIndex int, columnIndex int) bool {
	if rowIndex == 0 {
		if checked, _ := blk.Format["table_block_column_header"].(bool); checked {
			return true
		}
	}
	if columnIndex == 0 {
		if checked, _ := blk.Format["table_block_row_header"].(bool); checked {
			return true
		}
	}
	return false
}

func tableColumnFormats(blk block) map[string]map[string]any {
	raw, ok := blk.Format["table_block_column_format"].(map[string]any)
	if !ok || len(raw) == 0 {
		return nil
	}
	out := make(map[string]map[string]any, len(raw))
	for column, value := range raw {
		data, ok := value.(map[string]any)
		if !ok {
			continue
		}
		out[column] = data
	}
	return out
}

func notionTableCellColorClass(format map[string]any) string {
	if len(format) == 0 {
		return ""
	}
	color := notionColorClass(stringValue(format["color"]))
	if color == "" {
		return ""
	}
	return "notion-color--" + color
}

func notionTableColumnStyle(format map[string]any) string {
	if len(format) == 0 {
		return ""
	}
	width, ok := numberValue(format["width"])
	if !ok || width <= 0 || width > 2000 {
		return ""
	}
	return fmt.Sprintf("width:%.0fpx", width)
}

func tableColumnOrder(blk block) []string {
	raw, ok := blk.Format["table_block_column_order"].([]any)
	if !ok {
		return nil
	}
	columns := make([]string, 0, len(raw))
	for _, item := range raw {
		if column, ok := item.(string); ok && column != "" {
			columns = append(columns, column)
		}
	}
	return columns
}

func tableRowColumns(row block) []string {
	columns := make([]string, 0, len(row.Properties))
	for column := range row.Properties {
		columns = append(columns, column)
	}
	sort.Strings(columns)
	return columns
}
