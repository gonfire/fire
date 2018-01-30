package coal

import (
	"bytes"
	"fmt"
	"sort"
)

// Visualize emits a string in dot format that when put through graphviz
// visualizes the model dependencies.
func (c *Catalog) Visualize(title string) string {
	// prepare buffer
	var out bytes.Buffer

	// start graph
	out.WriteString("digraph G {\n")
	out.WriteString("  rankdir=\"LR\";\n")
	out.WriteString("  ranksep=\"0.5\";\n")
	out.WriteString("  nodesep=\"0.4\";\n")
	out.WriteString("  pad=\"0.4,0.4\";\n")
	out.WriteString("  margin=\"0,0\";\n")
	out.WriteString("  concentrate=\"true\";\n")
	out.WriteString("  labelloc=\"t\";\n")
	out.WriteString("  fontsize=\"13\";\n")
	out.WriteString("  fontname=\"Arial BoldMT\";\n")
	out.WriteString("  splines=\"spline\";\n")
	out.WriteString("  label=\"" + title + "\";\n")

	// get a sorted list of model names and lookup table
	var names []string
	lookup := make(map[string]string)
	for name, model := range c.models {
		names = append(names, name)
		lookup[name] = model.Meta().Name
	}
	sort.Strings(names)

	// add model nodes
	for _, name := range names {
		// get model
		model := c.models[name]

		// write begin of node
		out.WriteString(fmt.Sprintf(`  "%s" [ label=`, lookup[name]))

		// write head table
		out.WriteString(fmt.Sprintf(`<<table border="0" align="center" cellspacing="0.5" cellpadding="0" width="134"><tr><td align="center" valign="bottom" width="130"><font face="Arial BoldMT" point-size="11">%s</font></td></tr></table>|`, lookup[name]))

		// write begin of tail table
		out.WriteString(fmt.Sprintf(`<table border="0" align="left" cellspacing="2" cellpadding="0" width="134">`))

		// write attributes
		for _, field := range model.Meta().Fields {
			out.WriteString(fmt.Sprintf(`<tr><td align="left" width="130" port="%s">%s<font face="Arial ItalicMT" color="grey60"> %s</font></td></tr>`, field.Name, field.Name, field.Type.String()))
		}

		// write end of tail table
		out.WriteString(fmt.Sprintf(`</table>>`))

		// write end of node
		out.WriteString(`, shape=Mrecord, fontsize=10, fontname="ArialMT", margin="0.07,0.05", penwidth="1.0" ];` + "\n")
	}

	// define temporary struct
	type rel struct {
		from, to   string
		srcMany    bool
		dstMany    bool
		hasInverse bool
	}

	// prepare list
	list := make(map[string]*rel)
	var relNames []string

	// prepare relationships
	for _, name := range names {
		// get model
		model := c.models[name]

		// add all direct relationships
		for _, field := range model.Meta().Fields {
			if field.RelName != "" && (field.ToOne || field.ToMany) {
				list[name+"-"+field.RelName] = &rel{
					from:    name,
					to:      field.RelType,
					srcMany: field.ToMany,
				}

				relNames = append(relNames, name+"-"+field.RelName)
			}
		}
	}

	// update relationships
	for _, name := range names {
		// get model
		model := c.models[name]

		// add all indirect relationships
		for _, field := range model.Meta().Fields {
			if field.RelName != "" && (field.HasOne || field.HasMany) {
				r := list[field.RelType+"-"+field.RelInverse]
				r.dstMany = field.HasMany
				r.hasInverse = true
			}
		}
	}

	// sort relationship names
	sort.Strings(relNames)

	// add relationships
	for _, name := range relNames {
		// get relationship
		r := list[name]

		// get style
		style := "solid"
		if !r.hasInverse {
			style = "dotted"
		}

		// get arrow head
		arrowHead := "none"
		if r.dstMany {
			arrowHead = "normal"
		}

		// get arrow tail
		arrowTail := "none"
		if r.srcMany {
			arrowTail = "normal"
		}

		// write edge
		out.WriteString(fmt.Sprintf(`  "%s"->"%s"[ fontname="ArialMT", fontsize=7, dir=both, arrowsize="0.9", penwidth="0.9", labelangle=32, labeldistance="1.8", style=%s, arrowhead=%s, arrowtail=%s ];`, lookup[r.from], lookup[r.to], style, arrowHead, arrowTail) + "\n")
	}

	// end graph
	out.WriteString("}\n")

	return out.String()
}