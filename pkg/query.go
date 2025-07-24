package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/grafana/grafana-plugin-sdk-go/data"
)

// QueryModel represents the query from the frontend
type QueryModel struct {
	QueryType   string   `json:"queryType"`
	AQLQuery    string   `json:"aqlQuery,omitempty"`
	Collection  string   `json:"collection,omitempty"`
	Filter      string   `json:"filter,omitempty"`
	Limit       int      `json:"limit,omitempty"`
	SortBy      string   `json:"sortBy,omitempty"`
	SortOrder   string   `json:"sortOrder,omitempty"`
	TimeField   string   `json:"timeField,omitempty"`
	ValueField  string   `json:"valueField,omitempty"`
	Fields      []string `json:"fields,omitempty"`
	GroupBy     string   `json:"groupBy,omitempty"`
	Aggregation string   `json:"aggregation,omitempty"`
}

// buildCollectionQuery builds an AQL query from collection options
func (d *ArangoDBDatasource) buildCollectionQuery(qm *QueryModel, from, to int64) (string, error) {
	if qm.Collection == "" {
		return "", fmt.Errorf("collection is required for collection queries")
	}

	log.DefaultLogger.Debug("Building collection query", "collection", qm.Collection, "limit", qm.Limit, "filter", qm.Filter)

	var aql strings.Builder
	aql.WriteString(fmt.Sprintf("FOR doc IN %s", qm.Collection))

	// Add time filter if timeField is specified
	if qm.TimeField != "" {
		aql.WriteString(fmt.Sprintf(" FILTER doc.%s >= @from AND doc.%s <= @to", qm.TimeField, qm.TimeField))
	}

	// Add custom filter
	if qm.Filter != "" {
		// Process filter to automatically add "doc." prefix where needed
		processedFilter := processFilterExpression(qm.Filter)
		log.DefaultLogger.Debug("Filter processing", "original", qm.Filter, "processed", processedFilter)
		aql.WriteString(fmt.Sprintf(" FILTER %s", processedFilter))
	}

	// Add sorting
	if qm.SortBy != "" {
		sortOrder := "ASC"
		if qm.SortOrder == "DESC" {
			sortOrder = "DESC"
		}
		aql.WriteString(fmt.Sprintf(" SORT doc.%s %s", qm.SortBy, sortOrder))
	}

	// Add grouping
	if qm.GroupBy != "" {
		log.DefaultLogger.Debug("Adding GROUP BY clause", "groupBy", qm.GroupBy)
		// For grouping, we need to collect the documents first, then group them
		if qm.Aggregation != "" && qm.ValueField != "" {
			// Handle aggregation with grouping
			valueField := qm.ValueField
			if !strings.HasPrefix(valueField, "doc.") {
				valueField = "doc." + valueField
			}
			groupField := qm.GroupBy
			if !strings.HasPrefix(groupField, "doc.") {
				groupField = "doc." + groupField
			}
			
			switch qm.Aggregation {
			case "COUNT":
				aql.WriteString(fmt.Sprintf(" COLLECT group = %s WITH COUNT INTO length RETURN { group: group, value: length }", groupField))
			case "SUM":
				aql.WriteString(fmt.Sprintf(" COLLECT group = %s AGGREGATE total = SUM(%s) RETURN { group: group, value: total }", groupField, valueField))
			case "AVG":
				aql.WriteString(fmt.Sprintf(" COLLECT group = %s AGGREGATE avg = AVG(%s) RETURN { group: group, value: avg }", groupField, valueField))
			case "MIN":
				aql.WriteString(fmt.Sprintf(" COLLECT group = %s AGGREGATE min = MIN(%s) RETURN { group: group, value: min }", groupField, valueField))
			case "MAX":
				aql.WriteString(fmt.Sprintf(" COLLECT group = %s AGGREGATE max = MAX(%s) RETURN { group: group, value: max }", groupField, valueField))
			default:
				// For unknown aggregation with grouping, collect documents
				aql.WriteString(fmt.Sprintf(" COLLECT group = %s INTO items RETURN { group: group, items: items }", groupField))
			}
		} else {
			// Simple grouping without aggregation - collect documents by group
			groupField := qm.GroupBy
			if !strings.HasPrefix(groupField, "doc.") {
				groupField = "doc." + groupField
			}
			aql.WriteString(fmt.Sprintf(" COLLECT group = %s INTO items RETURN { group: group, items: items }", groupField))
		}
		
		finalQuery := aql.String()
		log.DefaultLogger.Debug("Built AQL query with grouping", "query", finalQuery)
		return finalQuery, nil
	}

	// Add limit (only if not using grouping)
	if qm.Limit > 0 {
		log.DefaultLogger.Debug("Adding LIMIT clause", "limit", qm.Limit)
		aql.WriteString(fmt.Sprintf(" LIMIT %d", qm.Limit))
	} else {
		log.DefaultLogger.Debug("No limit specified, using default of 100 for safety")
		aql.WriteString(" LIMIT 100") // Reasonable default to prevent huge queries
	}

	// Handle aggregation
	if qm.Aggregation != "" && qm.ValueField != "" {
		// Auto-prefix ValueField if not already prefixed
		valueField := qm.ValueField
		if !strings.HasPrefix(valueField, "doc.") {
			valueField = "doc." + valueField
		}
		
		switch qm.Aggregation {
		case "COUNT":
			return fmt.Sprintf("RETURN { value: LENGTH(%s) }", aql.String()+" RETURN doc"), nil
		case "SUM":
			aql.WriteString(fmt.Sprintf(" RETURN SUM(%s)", valueField))
		case "AVG":
			aql.WriteString(fmt.Sprintf(" RETURN AVG(%s)", valueField))
		case "MIN":
			aql.WriteString(fmt.Sprintf(" RETURN MIN(%s)", valueField))
		case "MAX":
			aql.WriteString(fmt.Sprintf(" RETURN MAX(%s)", valueField))
		default:
			aql.WriteString(d.buildReturnClause(qm))
		}
	} else {
		aql.WriteString(d.buildReturnClause(qm))
	}

	finalQuery := aql.String()
	log.DefaultLogger.Debug("Built AQL query", "query", finalQuery)
	return finalQuery, nil
}

// buildReturnClause builds the RETURN clause for AQL query
func (d *ArangoDBDatasource) buildReturnClause(qm *QueryModel) string {
	if len(qm.Fields) > 0 {
		log.DefaultLogger.Debug("Building return clause with fields", "fields", qm.Fields)
		fieldList := make([]string, len(qm.Fields))
		for i, field := range qm.Fields {
			// Handle special case where user wants the full document
			if field == "doc" {
				fieldList[i] = `"doc": doc`
				log.DefaultLogger.Debug("Adding full document field", "clause", fieldList[i])
			} else {
				// For nested fields like "Fields" or "Fields.InvoiceData", 
				// create a safe field name and reference the correct path
				safeFieldName := strings.ReplaceAll(field, ".", "_")
				fieldList[i] = fmt.Sprintf(`"%s": doc.%s`, safeFieldName, field)
				log.DefaultLogger.Debug("Adding nested field", "originalField", field, "safeFieldName", safeFieldName, "clause", fieldList[i])
			}
		}
		returnClause := fmt.Sprintf(" RETURN { %s }", strings.Join(fieldList, ", "))
		log.DefaultLogger.Debug("Built return clause", "clause", returnClause)
		return returnClause
	}
	return " RETURN doc"
}

// interpolateVariables interpolates variables in custom AQL queries
func (d *ArangoDBDatasource) interpolateVariables(query string, from, to int64) string {
	// Replace time variables with bind parameters
	query = strings.ReplaceAll(query, "$__timeFrom", "@from")
	query = strings.ReplaceAll(query, "$__timeTo", "@to")

	// Replace $__timeFilter(field) with field >= @from AND field <= @to
	timeFilterRegex := regexp.MustCompile(`\$__timeFilter\(([^)]+)\)`)
	query = timeFilterRegex.ReplaceAllStringFunc(query, func(match string) string {
		// Extract field name from $__timeFilter(field)
		field := strings.TrimSpace(strings.Trim(match, "$__timeFilter()"))
		return fmt.Sprintf("%s >= @from AND %s <= @to", field, field)
	})

	// Handle shorthand syntax like {doc, doc.Fields} -> {doc: doc, docFields: doc.Fields}
	query = d.processShorthandReturnSyntax(query)

	return query
}

// processShorthandReturnSyntax converts shorthand return syntax to valid AQL
func (d *ArangoDBDatasource) processShorthandReturnSyntax(query string) string {
	// Look for return statements with shorthand syntax like {doc, doc.Fields}
	returnPattern := regexp.MustCompile(`(?i)return\s*\{([^}]+)\}`)
	
	return returnPattern.ReplaceAllStringFunc(query, func(match string) string {
		log.DefaultLogger.Debug("Processing shorthand return syntax", "original", match)
		
		// Extract the content inside the braces
		start := strings.Index(match, "{")
		end := strings.LastIndex(match, "}")
		if start == -1 || end == -1 {
			return match
		}
		
		prefix := match[:start+1]
		suffix := match[end:]
		content := strings.TrimSpace(match[start+1:end])
		
		// Split by commas and process each field
		fields := strings.Split(content, ",")
		processedFields := make([]string, len(fields))
		
		for i, field := range fields {
			field = strings.TrimSpace(field)
			
			if field == "doc" {
				// Special case: just "doc" -> "doc: doc"
				processedFields[i] = "doc: doc"
				log.DefaultLogger.Debug("Processed doc field", "result", processedFields[i])
			} else if strings.HasPrefix(field, "doc.") {
				// Convert "doc.Fields" -> "docFields: doc.Fields"
				// Remove "doc." prefix and convert to camelCase field name
				fieldPath := field[4:] // Remove "doc." prefix
				safeFieldName := "doc" + d.toCamelCase(fieldPath)
				processedFields[i] = fmt.Sprintf("%s: %s", safeFieldName, field)
				log.DefaultLogger.Debug("Processed nested field", "original", field, "safeFieldName", safeFieldName, "result", processedFields[i])
			} else {
				// Keep as-is if it's already in proper format
				processedFields[i] = field
			}
		}
		
		result := prefix + strings.Join(processedFields, ", ") + suffix
		log.DefaultLogger.Debug("Processed return statement", "original", match, "result", result)
		return result
	})
}

// toCamelCase converts dot notation to camelCase (e.g., "Fields.PlanningList" -> "FieldsPlanningList")
// Note: This preserves array index notation like "Fields.PlanningList[0]" -> "FieldsPlanningList[0]"
func (d *ArangoDBDatasource) toCamelCase(dotNotation string) string {
	// Handle array index notation first
	arrayIndexPattern := regexp.MustCompile(`\[(\d+)\]`)
	arrayIndexSuffix := ""
	if arrayIndexPattern.MatchString(dotNotation) {
		matches := arrayIndexPattern.FindAllString(dotNotation, -1)
		if len(matches) > 0 {
			arrayIndexSuffix = matches[len(matches)-1] // Take the last array index
			dotNotation = arrayIndexPattern.ReplaceAllString(dotNotation, "")
		}
	}
	
	parts := strings.Split(dotNotation, ".")
	result := ""
	for _, part := range parts {
		if len(part) > 0 {
			result += strings.ToUpper(part[:1]) + part[1:]
		}
	}
	
	return result + arrayIndexSuffix
}

// processFilterExpression automatically adds "doc." prefix to field references in filter expressions
func processFilterExpression(filter string) string {
	// Don't process if filter already contains "doc."
	if strings.Contains(filter, "doc.") {
		return filter
	}
	
	// Pattern to match field references (including dotted paths like Fields.BookingID)
	// This matches: word characters, optionally followed by dot and more word characters
	fieldPattern := regexp.MustCompile(`\b([a-zA-Z_][a-zA-Z0-9_]*(?:\.[a-zA-Z_][a-zA-Z0-9_]*)*)\b`)
	
	result := fieldPattern.ReplaceAllStringFunc(filter, func(match string) string {
		// Skip if it's a known operator, function, or literal
		switch strings.ToUpper(match) {
		case "AND", "OR", "NOT", "IN", "NULL", "TRUE", "FALSE", "LIKE", "IS":
			return match
		}
		
		// Skip if it's a number
		if regexp.MustCompile(`^\d+$`).MatchString(match) {
			return match
		}
		
		// Skip if it's already prefixed or is a function call
		if strings.Contains(match, "(") || strings.HasPrefix(match, "doc.") {
			return match
		}
		
		// Add doc. prefix to field references
		return "doc." + match
	})
	
	return result
}

// transformResult transforms ArangoDB query result to Grafana data frames
func (d *ArangoDBDatasource) transformResult(result *QueryResult, qm *QueryModel, refID string) ([]*data.Frame, error) {
	log.DefaultLogger.Debug("Transforming result", "resultCount", len(result.Result), "refID", refID)
	
	if len(result.Result) == 0 {
		// Return empty frame
		frame := data.NewFrame(refID)
		return []*data.Frame{frame}, nil
	}

	// Handle aggregation results (single value)
	if qm.Aggregation != "" {
		if len(result.Result) == 1 {
			switch val := result.Result[0].(type) {
			case float64, int64, int:
				timeValues := []time.Time{time.Now()}
				floatValues := []float64{toFloat64(val)}
				
				frame := data.NewFrame(refID,
					data.NewField("time", nil, timeValues),
					data.NewField("value", nil, floatValues),
				)
				return []*data.Frame{frame}, nil
			}
		}
	}

	// Handle document results
	_, ok := result.Result[0].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected result format")
	}

	// Collect ALL field names from ALL documents to handle heterogeneous schemas
	// Use selective flattening based on query structure
	allFieldNames := make(map[string]bool)
	fieldTypes := make(map[string]string) // Track the type for each field
	
	log.DefaultLogger.Debug("Scanning all documents for field discovery with selective flattening", "documentCount", len(result.Result))
	
	for _, item := range result.Result {
		if itemMap, ok := item.(map[string]interface{}); ok {
			// Use selective flattening based on query type and structure
			flattenedFields := d.selectiveFlatten(itemMap, qm)
			for fieldPath, value := range flattenedFields {
				if !allFieldNames[fieldPath] {
					allFieldNames[fieldPath] = true
					// Determine type from first non-nil occurrence
					if value != nil {
						switch value.(type) {
						case float64, int64, int:
							if fieldTypes[fieldPath] == "" {
								fieldTypes[fieldPath] = "number"
							}
						case bool:
							if fieldTypes[fieldPath] == "" {
								fieldTypes[fieldPath] = "bool"
							}
						case string:
							if fieldTypes[fieldPath] == "" {
								if d.isDateString(fmt.Sprintf("%v", value)) || fieldPath == qm.TimeField {
									fieldTypes[fieldPath] = "time"
								} else {
									fieldTypes[fieldPath] = "string"
								}
							}
						default:
							if fieldTypes[fieldPath] == "" {
								fieldTypes[fieldPath] = "string"
							}
						}
					}
				}
			}
		}
	}

	// Convert map to sorted slice for consistent ordering
	fieldNames := make([]string, 0, len(allFieldNames))
	for fieldName := range allFieldNames {
		fieldNames = append(fieldNames, fieldName)
	}
	
	log.DefaultLogger.Debug("Discovered flattened fields", "fieldCount", len(fieldNames), "fields", fieldNames)

	// Create slices for each field based on discovered schema
	fieldData := make(map[string]interface{})
	for _, fieldName := range fieldNames {
		fieldType := fieldTypes[fieldName]
		switch fieldType {
		case "number":
			fieldData[fieldName] = make([]float64, len(result.Result))
		case "bool":
			fieldData[fieldName] = make([]bool, len(result.Result))
		case "time":
			fieldData[fieldName] = make([]*time.Time, len(result.Result))
		default: // "string" or unknown
			fieldData[fieldName] = make([]string, len(result.Result))
		}
	}

	// Populate field values - handle missing fields gracefully with selective flattening
	for i, item := range result.Result {
		if itemMap, ok := item.(map[string]interface{}); ok {
			// Use selective flattening for this document
			flattenedFields := d.selectiveFlatten(itemMap, qm)
			
			for _, fieldName := range fieldNames {
				value := flattenedFields[fieldName] // This will be nil if field doesn't exist in this document
				d.setFieldValueInSlice(fieldData, fieldName, i, value, qm.TimeField == fieldName)
			}
		}
	}

	// Create fields from populated data
	var fields []*data.Field
	for _, fieldName := range fieldNames {
		switch slice := fieldData[fieldName].(type) {
		case []float64:
			fields = append(fields, data.NewField(fieldName, nil, slice))
		case []bool:
			fields = append(fields, data.NewField(fieldName, nil, slice))
		case []*time.Time:
			fields = append(fields, data.NewField(fieldName, nil, slice))
		case []string:
			fields = append(fields, data.NewField(fieldName, nil, slice))
		}
	}

	frame := data.NewFrame(refID, fields...)
	log.DefaultLogger.Debug("Created data frame", "refID", refID, "fieldCount", len(fields), "rowCount", frame.Rows())
	return []*data.Frame{frame}, nil
}

// setFieldValueInSlice sets a value in the appropriate slice type, handling nil values gracefully
func (d *ArangoDBDatasource) setFieldValueInSlice(fieldData map[string]interface{}, fieldName string, index int, value interface{}, isTimeField bool) {
	switch slice := fieldData[fieldName].(type) {
	case []float64:
		if value == nil {
			slice[index] = 0 // Default value for missing numbers
		} else {
			slice[index] = toFloat64(value)
		}
	case []bool:
		if value == nil {
			slice[index] = false // Default value for missing booleans
		} else if val, ok := value.(bool); ok {
			slice[index] = val
		} else {
			slice[index] = false
		}
	case []*time.Time:
		if value == nil {
			slice[index] = nil // Nullable time allows nil
		} else {
			timeVal := d.parseTime(value, isTimeField)
			slice[index] = timeVal
		}
	case []string:
		if value == nil {
			slice[index] = "" // Default value for missing strings
		} else {
			slice[index] = toString(value)
		}
	}
}

// createField creates a data field based on the value type and returns both the field and slice
func (d *ArangoDBDatasource) createField(name string, value interface{}, length int) (*data.Field, interface{}) {
	switch val := value.(type) {
	case float64, int64, int:
		slice := make([]float64, length)
		return data.NewField(name, nil, slice), slice
	case bool:
		slice := make([]bool, length)
		return data.NewField(name, nil, slice), slice
	case string:
		// Check if it looks like a timestamp
		if d.isDateString(val) {
			slice := make([]*time.Time, length)
			return data.NewField(name, nil, slice), slice
		}
		slice := make([]string, length)
		return data.NewField(name, nil, slice), slice
	default:
		// Default to string
		slice := make([]string, length)
		return data.NewField(name, nil, slice), slice
	}
}

// setFieldValue sets a value in a data field
func (d *ArangoDBDatasource) setFieldValue(field *data.Field, index int, value interface{}, isTimeField bool) {
	switch field.Type() {
	case data.FieldTypeFloat64:
		if field.Len() > index {
			field.Set(index, toFloat64(value))
		}
	case data.FieldTypeBool:
		if field.Len() > index {
			if val, ok := value.(bool); ok {
				field.Set(index, val)
			}
		}
	case data.FieldTypeNullableTime:
		if field.Len() > index {
			timeVal := d.parseTime(value, isTimeField)
			field.Set(index, timeVal)
		}
	case data.FieldTypeString:
		if field.Len() > index {
			field.Set(index, toString(value))
		}
	}
}

// parseTime parses various time formats
func (d *ArangoDBDatasource) parseTime(value interface{}, isTimeField bool) *time.Time {
	switch val := value.(type) {
	case int64:
		// Unix timestamp (milliseconds or seconds)
		if val > 1e12 {
			// Milliseconds
			t := time.Unix(0, val*int64(time.Millisecond))
			return &t
		} else {
			// Seconds
			t := time.Unix(val, 0)
			return &t
		}
	case float64:
		// Unix timestamp
		if val > 1e12 {
			// Milliseconds
			t := time.Unix(0, int64(val)*int64(time.Millisecond))
			return &t
		} else {
			// Seconds
			t := time.Unix(int64(val), 0)
			return &t
		}
	case string:
		// Try parsing ISO format
		if t, err := time.Parse(time.RFC3339, val); err == nil {
			return &t
		}
		// Try other common formats
		formats := []string{
			"2006-01-02T15:04:05.000Z",
			"2006-01-02T15:04:05Z",
			"2006-01-02 15:04:05",
			"2006-01-02T15:04:05",
		}
		for _, format := range formats {
			if t, err := time.Parse(format, val); err == nil {
				return &t
			}
		}
	}
	return nil
}

// isDateString checks if a string looks like a date
func (d *ArangoDBDatasource) isDateString(value string) bool {
	// Check for ISO date format
	return regexp.MustCompile(`^\d{4}-\d{2}-\d{2}`).MatchString(value)
}

// Helper functions for type conversion
func toFloat64(value interface{}) float64 {
	if value == nil {
		return 0
	}
	switch val := value.(type) {
	case float64:
		return val
	case int64:
		return float64(val)
	case int:
		return float64(val)
	case string:
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return f
		}
	}
	return 0
}

func toString(value interface{}) string {
	if value == nil {
		return ""
	}
	return fmt.Sprintf("%v", value)
}

// flattenObject recursively flattens nested objects into dot-notation field paths
func (d *ArangoDBDatasource) flattenObject(obj map[string]interface{}, prefix string) map[string]interface{} {
	flattened := make(map[string]interface{})
	
	for key, value := range obj {
		var newKey string
		if prefix == "" {
			newKey = key
		} else {
			newKey = prefix + "." + key
		}
		
		// Check if value is an array
		if arrayValue, ok := value.([]interface{}); ok {
			// Flatten array elements with index suffixes
			log.DefaultLogger.Debug("Flattening array", "key", newKey, "arrayLength", len(arrayValue))
			for i, arrayItem := range arrayValue {
				arrayKey := fmt.Sprintf("%s[%d]", newKey, i)
				if arrayItemObj, ok := arrayItem.(map[string]interface{}); ok {
					// Recursively flatten array item object
					arrayFlattened := d.flattenObject(arrayItemObj, arrayKey)
					for arrayFlatKey, arrayFlatValue := range arrayFlattened {
						flattened[arrayFlatKey] = arrayFlatValue
					}
				} else {
					// It's a primitive value in the array
					flattened[arrayKey] = arrayItem
				}
			}
		} else if nestedObj, ok := value.(map[string]interface{}); ok {
			// Recursively flatten nested object
			log.DefaultLogger.Debug("Flattening nested object", "key", newKey, "nestedKeys", len(nestedObj))
			nestedFlattened := d.flattenObject(nestedObj, newKey)
			for nestedKey, nestedValue := range nestedFlattened {
				flattened[nestedKey] = nestedValue
			}
		} else {
			// It's a primitive value, add it directly
			flattened[newKey] = value
		}
	}
	
	return flattened
}

// selectiveFlatten applies smart flattening based on query type and structure
func (d *ArangoDBDatasource) selectiveFlatten(obj map[string]interface{}, qm *QueryModel) map[string]interface{} {
	// For custom AQL queries, detect what was requested by analyzing the structure
	if qm.QueryType == "aql" {
		return d.analyzeAndFlattenAQL(obj)
	}
	
	// For collection queries, use the specified fields to determine flattening behavior
	if len(qm.Fields) > 0 {
		return d.flattenBasedOnFields(obj, qm.Fields)
	}
	
	// Default: when returning full document, only flatten first level
	return d.flattenTopLevelOnly(obj)
}

// analyzeAndFlattenAQL analyzes AQL result structure to determine flattening strategy
func (d *ArangoDBDatasource) analyzeAndFlattenAQL(obj map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	
	for key, value := range obj {
		if key == "doc" {
			// Handle the special "doc" field - extract all root level fields
			if docObj, ok := value.(map[string]interface{}); ok {
				log.DefaultLogger.Debug("AQL: Processing doc field", "rootKeys", len(docObj))
				for docKey, docValue := range docObj {
					// Only add root level fields (not nested objects or arrays) - convert complex types to string
					if _, isNested := docValue.(map[string]interface{}); !isNested {
						if _, isArray := docValue.([]interface{}); !isArray {
							result[docKey] = docValue
						} else {
							result[docKey] = fmt.Sprintf("%v", docValue)
						}
					} else {
						result[docKey] = fmt.Sprintf("%v", docValue)
					}
				}
			}
		} else if strings.HasPrefix(key, "doc") && len(key) > 3 {
			// Handle fields like "docFields", "docFieldsPlanningList" etc.
			
			// Convert camelCase back to dot notation for flattening prefix
			// "docFields" -> "Fields", "docFieldsPlanningList" -> "Fields.PlanningList"
			fieldPath := d.fromCamelCaseToPath(key[3:]) // Remove "doc" prefix
			
			if nestedObj, ok := value.(map[string]interface{}); ok {
				log.DefaultLogger.Debug("AQL: Flattening nested object", "key", key, "fieldPath", fieldPath, "nestedKeys", len(nestedObj))
				flattened := d.flattenObject(nestedObj, fieldPath)
				for flatKey, flatValue := range flattened {
					result[flatKey] = flatValue
				}
			} else if arrayValue, ok := value.([]interface{}); ok {
				// Handle arrays directly
				log.DefaultLogger.Debug("AQL: Flattening array", "key", key, "fieldPath", fieldPath, "arrayLength", len(arrayValue))
				for i, arrayItem := range arrayValue {
					arrayKey := fmt.Sprintf("%s[%d]", fieldPath, i)
					if arrayItemObj, ok := arrayItem.(map[string]interface{}); ok {
						// Recursively flatten array item object
						arrayFlattened := d.flattenObject(arrayItemObj, arrayKey)
						for arrayFlatKey, arrayFlatValue := range arrayFlattened {
							result[arrayFlatKey] = arrayFlatValue
						}
					} else {
						// It's a primitive value in the array
						result[arrayKey] = arrayItem
					}
				}
			} else {
				// It's a primitive value from a nested field
				result[fieldPath] = value
			}
		} else {
			// Any other field, keep as-is (shouldn't happen with our preprocessing)
			if nestedObj, ok := value.(map[string]interface{}); ok {
				log.DefaultLogger.Debug("AQL: Flattening other nested object", "key", key, "nestedKeys", len(nestedObj))
				flattened := d.flattenObject(nestedObj, key)
				for flatKey, flatValue := range flattened {
					result[flatKey] = flatValue
				}
			} else if arrayValue, ok := value.([]interface{}); ok {
				log.DefaultLogger.Debug("AQL: Flattening other array", "key", key, "arrayLength", len(arrayValue))
				for i, arrayItem := range arrayValue {
					arrayKey := fmt.Sprintf("%s[%d]", key, i)
					if arrayItemObj, ok := arrayItem.(map[string]interface{}); ok {
						arrayFlattened := d.flattenObject(arrayItemObj, arrayKey)
						for arrayFlatKey, arrayFlatValue := range arrayFlattened {
							result[arrayFlatKey] = arrayFlatValue
						}
					} else {
						result[arrayKey] = arrayItem
					}
				}
			} else {
				result[key] = value
			}
		}
	}
	
	return result
}

// fromCamelCaseToPath converts camelCase back to dot notation (e.g., "FieldsPlanningList" -> "Fields.PlanningList")
// Also handles array index notation like "FieldsPlanningList[0]" -> "Fields.PlanningList[0]"
func (d *ArangoDBDatasource) fromCamelCaseToPath(camelCase string) string {
	if camelCase == "" {
		return ""
	}
	
	// Handle array index notation first
	arrayIndexPattern := regexp.MustCompile(`\[(\d+)\]`)
	arrayIndexSuffix := ""
	if arrayIndexPattern.MatchString(camelCase) {
		matches := arrayIndexPattern.FindAllString(camelCase, -1)
		if len(matches) > 0 {
			arrayIndexSuffix = matches[len(matches)-1] // Take the last array index
			camelCase = arrayIndexPattern.ReplaceAllString(camelCase, "")
		}
	}
	
	// Split on uppercase letters
	var result []string
	var current strings.Builder
	
	for i, r := range camelCase {
		if i > 0 && r >= 'A' && r <= 'Z' {
			if current.Len() > 0 {
				result = append(result, current.String())
				current.Reset()
			}
		}
		current.WriteRune(r)
	}
	
	if current.Len() > 0 {
		result = append(result, current.String())
	}
	
	return strings.Join(result, ".") + arrayIndexSuffix
}

// flattenBasedOnFields flattens only the fields specified in the Return Fields
func (d *ArangoDBDatasource) flattenBasedOnFields(obj map[string]interface{}, fields []string) map[string]interface{} {
	result := make(map[string]interface{})
	
	// Process each requested field
	for _, originalField := range fields {
		// Handle the special "doc" field - this means include all root level fields
		if originalField == "doc" {
			if docValue, exists := obj["doc"]; exists {
				if docObj, ok := docValue.(map[string]interface{}); ok {
					// Add all root level fields from the doc object
					log.DefaultLogger.Debug("Collection: Including root level fields from doc", "rootKeys", len(docObj))
					for key, value := range docObj {
						// Only add root level fields (not nested objects)
						if _, isNested := value.(map[string]interface{}); !isNested {
							result[key] = value
						} else {
							// For nested objects, convert to string representation
							result[key] = fmt.Sprintf("%v", value)
						}
					}
				}
			}
			continue
		}
		
		// For nested field requests like "Fields" or "Fields.InvoiceData"
		// The field name in the result object will be sanitized (dots replaced with underscores)
		safeFieldName := strings.ReplaceAll(originalField, ".", "_")
		
		if value, exists := obj[safeFieldName]; exists {
			if nestedObj, ok := value.(map[string]interface{}); ok {
				// This field is a nested object, flatten it using the original field name as prefix
				log.DefaultLogger.Debug("Collection: Flattening specified field", "originalField", originalField, "safeFieldName", safeFieldName, "nestedKeys", len(nestedObj))
				flattened := d.flattenObject(nestedObj, originalField)
				for flatKey, flatValue := range flattened {
					result[flatKey] = flatValue
				}
			} else {
				// It's a primitive, keep as-is with the original field name
				result[originalField] = value
			}
		}
	}
	
	return result
}

// flattenTopLevelOnly keeps nested objects as JSON strings instead of flattening
func (d *ArangoDBDatasource) flattenTopLevelOnly(obj map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	
	for key, value := range obj {
		if nestedObj, ok := value.(map[string]interface{}); ok {
			// Convert nested object to JSON string representation
			log.DefaultLogger.Debug("Keeping nested object as string", "key", key, "nestedKeys", len(nestedObj))
			result[key] = fmt.Sprintf("%v", nestedObj)
		} else {
			// Keep primitive values as-is
			result[key] = value
		}
	}
	
	return result
}
