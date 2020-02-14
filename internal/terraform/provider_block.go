package terraform

import (
	"fmt"
	"strings"

	hcl "github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	tfjson "github.com/hashicorp/terraform-json"
	lsp "github.com/sourcegraph/go-lsp"
)

func ProviderBlock(block *hcl.Block) (ConfigBlock, error) {
	labels := block.Labels
	if len(labels) != 1 {
		return nil, fmt.Errorf("unexpected labels for provider block: %q", labels)
	}

	return &providerBlock{hclBlock: block}, nil
}

type providerBlock struct {
	hclBlock *hcl.Block
	schema   *tfjson.Schema
}

func (p *providerBlock) CompletionItemsAtPos(pos hcl.Pos) (lsp.CompletionList, error) {
	list := lsp.CompletionList{}

	if p.schema == nil {
		return list, fmt.Errorf("schema unavailable for provider %q", p.Name())
	}

	hs := jsonSchemaToHcl(p.schema)

	content, body, diags := p.hclBlock.Body.PartialContent(hs)
	if diags.HasErrors() {
		return list, diags
	}

	hclBody, ok := body.(*hclsyntax.Body)
	if !ok {
		// if user happens to be editing JSON
		return list, fmt.Errorf("unsupported body type: %T", body)
	}

	if !bodyContainsPos(hclBody, pos) {
		// Avoid autocompleting outside of body, for now
		return list, nil
	}

	if contentContainPos(hclBody, pos) {
		// No auto-completing in the middle of existing fields
		return list, nil
	}

	attrs := undeclaredSchemaAttributes(p.schema.Block.Attributes, content.Attributes)
	// TODO: blocks := undeclaredSchemaBlocks(p.schema.Block.NestedBlocks, content.Blocks)

	for name, attr := range attrs {
		if attr.Computed && !attr.Optional && !attr.Required {
			continue
		}

		list.Items = append(list.Items, lsp.CompletionItem{
			Label:  name,
			Kind:   lsp.CIKField,
			Detail: schemaAttributeDetail(attr),
		})
	}

	return list, nil
}

func schemaAttributeDetail(attr *tfjson.SchemaAttribute) string {
	var requiredText string
	if attr.Optional {
		requiredText = "Optional"
	}
	if attr.Required {
		requiredText = "Required"
	}

	return strings.TrimSpace(fmt.Sprintf("(%s, %s) %s",
		requiredText, attr.AttributeType.FriendlyName(), attr.Description))
}

func (p *providerBlock) Name() string {
	return p.hclBlock.Labels[0]
}

func (p *providerBlock) LoadSchema(ps *tfjson.ProviderSchemas) error {
	providerName := p.hclBlock.Labels[0]

	schema, ok := ps.Schemas[providerName]
	if !ok {
		return fmt.Errorf("schema not found for provider %q", providerName)
	}

	p.schema = schema.ConfigSchema
	return nil
}