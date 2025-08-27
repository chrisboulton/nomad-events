package outputs

import (
	"fmt"
	"reflect"
	"strings"

	"nomad-events/internal/nomad"
	"nomad-events/internal/template"

	"github.com/slack-go/slack"
)

type SlackTemplateEngine struct {
	engine *template.Engine
}

type BlockConfig struct {
	Type     string        `yaml:"type"`
	Text     interface{}   `yaml:"text,omitempty"`
	Fields   []interface{} `yaml:"fields,omitempty"`
	Elements []interface{} `yaml:"elements,omitempty"`
	Options  []interface{} `yaml:"options,omitempty"`
	ImageURL string        `yaml:"image_url,omitempty"`
	AltText  string        `yaml:"alt_text,omitempty"`
	Title    interface{}   `yaml:"title,omitempty"`
	Label    interface{}   `yaml:"label,omitempty"`
	Hint     interface{}   `yaml:"hint,omitempty"`
	Optional bool          `yaml:"optional,omitempty"`
	BlockID  string        `yaml:"block_id,omitempty"`
}

type TextConfig struct {
	Type  string `yaml:"type"`
	Text  string `yaml:"text"`
	Emoji bool   `yaml:"emoji,omitempty"`
}

type ElementConfig struct {
	Type        string        `yaml:"type"`
	Text        interface{}   `yaml:"text,omitempty"`
	URL         string        `yaml:"url,omitempty"`
	Value       string        `yaml:"value,omitempty"`
	ActionID    string        `yaml:"action_id,omitempty"`
	Placeholder interface{}   `yaml:"placeholder,omitempty"`
	Options     []interface{} `yaml:"options,omitempty"`
}

type OptionConfig struct {
	Text  interface{} `yaml:"text"`
	Value string      `yaml:"value"`
}

func NewSlackTemplateEngine() *SlackTemplateEngine {
	return &SlackTemplateEngine{
		engine: template.NewEngine(),
	}
}

func (ste *SlackTemplateEngine) ProcessBlocks(blockConfigs []BlockConfig, event nomad.Event) ([]slack.Block, error) {
	var blocks []slack.Block

	eventData := ste.engine.CreateTemplateData(event)

	for _, blockConfig := range blockConfigs {
		if isRange, rangePath := ste.isRangeItem(blockConfig); isRange {
			expandedBlocks, err := ste.expandRangeItem(blockConfig, rangePath, eventData, func(templateItem interface{}, itemData map[string]interface{}) (interface{}, error) {
				if blockConfig, ok := templateItem.(BlockConfig); ok {
					return ste.processBlock(blockConfig, itemData)
				}
				return nil, fmt.Errorf("invalid block config type")
			})
			if err != nil {
				return nil, fmt.Errorf("failed to process range blocks: %w", err)
			}
			for _, expandedBlock := range expandedBlocks {
				if block, ok := expandedBlock.(slack.Block); ok {
					blocks = append(blocks, block)
				}
			}
		} else {
			block, err := ste.processBlock(blockConfig, eventData)
			if err != nil {
				return nil, fmt.Errorf("failed to process block: %w", err)
			}
			if block != nil {
				blocks = append(blocks, block)
			}
		}
	}

	return blocks, nil
}

func (ste *SlackTemplateEngine) ProcessText(text string, event nomad.Event) (string, error) {
	return ste.engine.ProcessText(text, event)
}

func (ste *SlackTemplateEngine) processBlock(blockConfig BlockConfig, eventData map[string]interface{}) (slack.Block, error) {
	switch blockConfig.Type {
	case "header":
		return ste.createHeaderBlock(blockConfig, eventData)
	case "divider":
		return ste.createDividerBlock(blockConfig)
	case "section":
		return ste.createSectionBlock(blockConfig, eventData)
	case "context":
		return ste.createContextBlock(blockConfig, eventData)
	case "actions":
		return ste.createActionBlock(blockConfig, eventData)
	case "image":
		return ste.createImageBlock(blockConfig, eventData)
	case "input":
		return ste.createInputBlock(blockConfig, eventData)
	default:
		return nil, fmt.Errorf("unsupported block type: %s", blockConfig.Type)
	}
}

func (ste *SlackTemplateEngine) createHeaderBlock(blockConfig BlockConfig, eventData map[string]interface{}) (slack.Block, error) {
	text, err := ste.processText(blockConfig.Text, eventData)
	if err != nil {
		return nil, err
	}

	textObj := slack.NewTextBlockObject(slack.PlainTextType, text, false, false)
	return slack.NewHeaderBlock(textObj), nil
}

func (ste *SlackTemplateEngine) createDividerBlock(blockConfig BlockConfig) (slack.Block, error) {
	return slack.NewDividerBlock(), nil
}

func (ste *SlackTemplateEngine) createSectionBlock(blockConfig BlockConfig, eventData map[string]interface{}) (slack.Block, error) {
	var textObj *slack.TextBlockObject
	var fields []*slack.TextBlockObject

	if blockConfig.Text != nil {
		textConfig, err := ste.parseTextConfig(blockConfig.Text, eventData)
		if err != nil {
			return nil, err
		}
		textObj = slack.NewTextBlockObject(textConfig.Type, textConfig.Text, textConfig.Emoji, false)
	}

	processedFields, err := ste.processFields(blockConfig.Fields, eventData)
	if err == nil {
		fields = processedFields
	}

	return slack.NewSectionBlock(textObj, fields, nil), nil
}

func (ste *SlackTemplateEngine) processFields(fieldsConfig []interface{}, eventData map[string]interface{}) ([]*slack.TextBlockObject, error) {
	var fields []*slack.TextBlockObject

	for _, field := range fieldsConfig {
		if isRange, rangePath := ste.isRangeItem(field); isRange {
			expandedFields, err := ste.expandRangeItem(field, rangePath, eventData, ste.processTextField)
			if err != nil {
				continue
			}
			for _, expandedField := range expandedFields {
				if textObj, ok := expandedField.(*slack.TextBlockObject); ok {
					fields = append(fields, textObj)
				}
			}
		} else {
			fieldObj, err := ste.processSingleTextField(field, eventData)
			if err == nil && fieldObj != nil {
				fields = append(fields, fieldObj)
			}
		}
	}

	return fields, nil
}

func (ste *SlackTemplateEngine) processTextField(templateItem interface{}, itemData map[string]interface{}) (interface{}, error) {
	return ste.processSingleTextField(templateItem, itemData)
}

func (ste *SlackTemplateEngine) processSingleTextField(field interface{}, eventData map[string]interface{}) (*slack.TextBlockObject, error) {
	fieldConfig, err := ste.parseTextConfig(field, eventData)
	if err != nil {
		return nil, err
	}
	return slack.NewTextBlockObject(fieldConfig.Type, fieldConfig.Text, fieldConfig.Emoji, false), nil
}

func (ste *SlackTemplateEngine) createContextBlock(blockConfig BlockConfig, eventData map[string]interface{}) (slack.Block, error) {
	processedElements, err := ste.processContextElements(blockConfig.Elements, eventData)
	if err != nil {
		return nil, err
	}

	return slack.NewContextBlock(blockConfig.BlockID, processedElements...), nil
}

func (ste *SlackTemplateEngine) processContextElements(elementsConfig []interface{}, eventData map[string]interface{}) ([]slack.MixedElement, error) {
	var elements []slack.MixedElement

	for _, elem := range elementsConfig {
		if isRange, rangePath := ste.isRangeItem(elem); isRange {
			expandedElements, err := ste.expandRangeItem(elem, rangePath, eventData, ste.processContextElement)
			if err != nil {
				continue
			}
			for _, expandedElement := range expandedElements {
				if mixedElement, ok := expandedElement.(slack.MixedElement); ok {
					elements = append(elements, mixedElement)
				}
			}
		} else {
			element, err := ste.processSingleContextElement(elem, eventData)
			if err == nil && element != nil {
				elements = append(elements, element)
			}
		}
	}

	return elements, nil
}

func (ste *SlackTemplateEngine) processContextElement(templateItem interface{}, itemData map[string]interface{}) (interface{}, error) {
	return ste.processSingleContextElement(templateItem, itemData)
}

func (ste *SlackTemplateEngine) processSingleContextElement(elem interface{}, eventData map[string]interface{}) (slack.MixedElement, error) {
	textConfig, err := ste.parseTextConfig(elem, eventData)
	if err != nil {
		return nil, err
	}
	return slack.NewTextBlockObject(textConfig.Type, textConfig.Text, textConfig.Emoji, false), nil
}

func (ste *SlackTemplateEngine) createActionBlock(blockConfig BlockConfig, eventData map[string]interface{}) (slack.Block, error) {
	processedElements, err := ste.processActionElements(blockConfig.Elements, eventData)
	if err != nil {
		return nil, err
	}

	return slack.NewActionBlock(blockConfig.BlockID, processedElements...), nil
}

func (ste *SlackTemplateEngine) processActionElements(elementsConfig []interface{}, eventData map[string]interface{}) ([]slack.BlockElement, error) {
	var elements []slack.BlockElement

	for _, elem := range elementsConfig {
		if isRange, rangePath := ste.isRangeItem(elem); isRange {
			expandedElements, err := ste.expandRangeItem(elem, rangePath, eventData, ste.processActionElement)
			if err != nil {
				continue
			}
			for _, expandedElement := range expandedElements {
				if blockElement, ok := expandedElement.(slack.BlockElement); ok {
					elements = append(elements, blockElement)
				}
			}
		} else {
			element, err := ste.processSingleActionElement(elem, eventData)
			if err == nil && element != nil {
				elements = append(elements, element)
			}
		}
	}

	return elements, nil
}

func (ste *SlackTemplateEngine) processActionElement(templateItem interface{}, itemData map[string]interface{}) (interface{}, error) {
	return ste.processSingleActionElement(templateItem, itemData)
}

func (ste *SlackTemplateEngine) processSingleActionElement(elem interface{}, eventData map[string]interface{}) (slack.BlockElement, error) {
	return ste.processElement(elem, eventData)
}

func (ste *SlackTemplateEngine) createImageBlock(blockConfig BlockConfig, eventData map[string]interface{}) (slack.Block, error) {
	imageURL, err := ste.processText(blockConfig.ImageURL, eventData)
	if err != nil {
		return nil, err
	}

	altText, err := ste.processText(blockConfig.AltText, eventData)
	if err != nil {
		return nil, err
	}

	var titleObj *slack.TextBlockObject
	if blockConfig.Title != nil {
		titleText, err := ste.processText(blockConfig.Title, eventData)
		if err == nil {
			titleObj = slack.NewTextBlockObject(slack.PlainTextType, titleText, false, false)
		}
	}

	return slack.NewImageBlock(imageURL, altText, blockConfig.BlockID, titleObj), nil
}

func (ste *SlackTemplateEngine) createInputBlock(blockConfig BlockConfig, eventData map[string]interface{}) (slack.Block, error) {
	return nil, fmt.Errorf("input blocks not implemented yet")
}

func (ste *SlackTemplateEngine) processElement(elemConfig interface{}, eventData map[string]interface{}) (slack.BlockElement, error) {
	elemMap, ok := elemConfig.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid element configuration")
	}

	elemType, ok := elemMap["type"].(string)
	if !ok {
		return nil, fmt.Errorf("element type is required")
	}

	switch elemType {
	case "button":
		return ste.createButtonElement(elemMap, eventData)
	case "static_select":
		return ste.createStaticSelectElement(elemMap, eventData)
	default:
		return nil, fmt.Errorf("unsupported element type: %s", elemType)
	}
}

func (ste *SlackTemplateEngine) createButtonElement(elemMap map[string]interface{}, eventData map[string]interface{}) (slack.BlockElement, error) {
	textConfig, err := ste.parseTextConfig(elemMap["text"], eventData)
	if err != nil {
		return nil, err
	}

	textObj := slack.NewTextBlockObject(textConfig.Type, textConfig.Text, textConfig.Emoji, false)

	actionID, _ := elemMap["action_id"].(string)
	value, _ := elemMap["value"].(string)
	url, _ := elemMap["url"].(string)

	if actionID != "" {
		processedActionID, err := ste.processText(actionID, eventData)
		if err == nil {
			actionID = processedActionID
		}
	}

	if url != "" {
		processedURL, err := ste.processText(url, eventData)
		if err == nil {
			url = processedURL
		}
	}

	if value != "" {
		processedValue, err := ste.processText(value, eventData)
		if err == nil {
			value = processedValue
		}
	}

	btn := slack.NewButtonBlockElement(actionID, value, textObj)
	if url != "" {
		btn.URL = url
	}

	return btn, nil
}

func (ste *SlackTemplateEngine) createStaticSelectElement(elemMap map[string]interface{}, eventData map[string]interface{}) (slack.BlockElement, error) {
	var placeholder *slack.TextBlockObject
	if placeholderConfig, ok := elemMap["placeholder"]; ok {
		textConfig, err := ste.parseTextConfig(placeholderConfig, eventData)
		if err == nil {
			placeholder = slack.NewTextBlockObject(textConfig.Type, textConfig.Text, textConfig.Emoji, false)
		}
	}

	var options []*slack.OptionBlockObject
	if optionsConfig, ok := elemMap["options"].([]interface{}); ok {
		processedOptions, err := ste.processSelectOptions(optionsConfig, eventData)
		if err == nil {
			options = processedOptions
		}
	}

	actionID, _ := elemMap["action_id"].(string)

	return slack.NewOptionsSelectBlockElement(slack.OptTypeStatic, placeholder, actionID, options...), nil
}

func (ste *SlackTemplateEngine) processSelectOptions(optionsConfig []interface{}, eventData map[string]interface{}) ([]*slack.OptionBlockObject, error) {
	var options []*slack.OptionBlockObject

	for _, option := range optionsConfig {
		if isRange, rangePath := ste.isRangeItem(option); isRange {
			expandedOptions, err := ste.expandRangeItem(option, rangePath, eventData, ste.processSelectOption)
			if err != nil {
				continue
			}
			for _, expandedOption := range expandedOptions {
				if optionObj, ok := expandedOption.(*slack.OptionBlockObject); ok {
					options = append(options, optionObj)
				}
			}
		} else {
			optionObj, err := ste.processSingleSelectOption(option, eventData)
			if err == nil && optionObj != nil {
				options = append(options, optionObj)
			}
		}
	}

	return options, nil
}

func (ste *SlackTemplateEngine) processSelectOption(templateItem interface{}, itemData map[string]interface{}) (interface{}, error) {
	return ste.processSingleSelectOption(templateItem, itemData)
}

func (ste *SlackTemplateEngine) processSingleSelectOption(option interface{}, eventData map[string]interface{}) (*slack.OptionBlockObject, error) {
	return ste.createOption(option, eventData)
}

func (ste *SlackTemplateEngine) createOption(optionConfig interface{}, eventData map[string]interface{}) (*slack.OptionBlockObject, error) {
	optionMap, ok := optionConfig.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid option configuration")
	}

	textConfig, err := ste.parseTextConfig(optionMap["text"], eventData)
	if err != nil {
		return nil, err
	}

	textObj := slack.NewTextBlockObject(textConfig.Type, textConfig.Text, textConfig.Emoji, false)

	value, _ := optionMap["value"].(string)
	processedValue, err := ste.processText(value, eventData)
	if err == nil {
		value = processedValue
	}

	return slack.NewOptionBlockObject(value, textObj, nil), nil
}

func (ste *SlackTemplateEngine) parseTextConfig(textConfig interface{}, eventData map[string]interface{}) (*TextConfig, error) {
	if textStr, ok := textConfig.(string); ok {
		processedText, err := ste.processText(textStr, eventData)
		if err != nil {
			return nil, err
		}
		return &TextConfig{
			Type: slack.MarkdownType,
			Text: processedText,
		}, nil
	}

	if textMap, ok := textConfig.(map[string]interface{}); ok {
		textType, _ := textMap["type"].(string)
		if textType == "" {
			textType = slack.MarkdownType
		}

		textValue, _ := textMap["text"].(string)
		processedText, err := ste.processText(textValue, eventData)
		if err != nil {
			return nil, err
		}

		emoji, _ := textMap["emoji"].(bool)

		return &TextConfig{
			Type:  textType,
			Text:  processedText,
			Emoji: emoji,
		}, nil
	}

	return nil, fmt.Errorf("invalid text configuration")
}

func (ste *SlackTemplateEngine) processText(text interface{}, eventData map[string]interface{}) (string, error) {
	textStr, ok := text.(string)
	if !ok {
		return "", fmt.Errorf("text must be a string")
	}

	return ste.engine.ProcessTextWithData(textStr, eventData)
}


func (ste *SlackTemplateEngine) getNestedValue(data map[string]interface{}, path string) (interface{}, error) {
	parts := strings.Split(path, ".")
	current := data

	for i, part := range parts {
		if i == len(parts)-1 {
			value, exists := current[part]
			if !exists {
				return nil, fmt.Errorf("path not found: %s", path)
			}
			return value, nil
		}

		next, ok := current[part]
		if !ok {
			return nil, fmt.Errorf("path not found: %s", path)
		}

		nextMap, ok := next.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("path not found: %s", path)
		}

		current = nextMap
	}

	return nil, fmt.Errorf("path not found: %s", path)
}

func (ste *SlackTemplateEngine) isRangeItem(item interface{}) (bool, string) {
	if itemMap, ok := item.(map[string]interface{}); ok {
		if rangeVal, exists := itemMap["range"].(string); exists {
			return true, rangeVal
		}
	}
	return false, ""
}

func (ste *SlackTemplateEngine) createTemplateItem(rangeItem interface{}) interface{} {
	if itemMap, ok := rangeItem.(map[string]interface{}); ok {
		template := make(map[string]interface{})
		for k, v := range itemMap {
			if k != "range" {
				template[k] = v
			}
		}
		return template
	}
	return rangeItem
}

func (ste *SlackTemplateEngine) convertToSlice(data interface{}) []interface{} {
	if slice, ok := data.([]interface{}); ok {
		return slice
	}

	rv := reflect.ValueOf(data)
	if rv.Kind() != reflect.Slice && rv.Kind() != reflect.Array {
		return nil
	}

	slice := make([]interface{}, rv.Len())
	for i := 0; i < rv.Len(); i++ {
		slice[i] = rv.Index(i).Interface()
	}
	return slice
}

func (ste *SlackTemplateEngine) createItemContext(baseData map[string]interface{}, item interface{}) map[string]interface{} {
	itemData := make(map[string]interface{})
	for k, v := range baseData {
		itemData[k] = v
	}

	if itemMap, ok := item.(map[string]interface{}); ok {
		for k, v := range itemMap {
			itemData[k] = v
		}
	} else {
		itemData["Item"] = item
	}

	return itemData
}

func (ste *SlackTemplateEngine) expandRangeItem(
	rangeItem interface{},
	rangePath string,
	eventData map[string]interface{},
	itemProcessor func(interface{}, map[string]interface{}) (interface{}, error),
) ([]interface{}, error) {
	rangeData, err := ste.getNestedValue(eventData, strings.TrimPrefix(rangePath, "."))
	if err != nil {
		return nil, err
	}

	rangeSlice := ste.convertToSlice(rangeData)
	if rangeSlice == nil {
		return nil, fmt.Errorf("range data is not iterable")
	}

	var results []interface{}

	for _, item := range rangeSlice {
		itemContext := ste.createItemContext(eventData, item)
		templateItem := ste.createTemplateItem(rangeItem)

		processedItem, err := itemProcessor(templateItem, itemContext)
		if err == nil && processedItem != nil {
			results = append(results, processedItem)
		}
	}

	return results, nil
}
