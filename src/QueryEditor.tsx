import React, { PureComponent, ChangeEvent } from 'react';
import { QueryEditorProps, SelectableValue, DataSourceApi } from '@grafana/data';
import {
  InlineField,
  Input,
  TextArea,
  RadioButtonGroup,
  InlineFieldRow,
  Select,
  Button,
} from '@grafana/ui';
import { ArangoDBQuery, ArangoDBDataSourceOptions } from './types';

type Props = QueryEditorProps<DataSourceApi<ArangoDBQuery, ArangoDBDataSourceOptions>, ArangoDBQuery, ArangoDBDataSourceOptions>;

interface State {
  fieldsInput: string;
}

export class QueryEditor extends PureComponent<Props, State> {
  constructor(props: Props) {
    super(props);
    this.state = {
      fieldsInput: props.query.fields?.join(', ') || '',
    };
  }

  componentDidMount() {
    // Ensure query has default values
    const { query, onChange } = this.props;
    if (!query.queryType) {
      onChange({
        ...query,
        queryType: 'collection',
        refId: query.refId || 'A',
      });
    }
  }

  componentDidUpdate(prevProps: Props) {
    // Update local state if fields changed externally
    const currentFields = this.props.query.fields?.join(', ') || '';
    const prevFields = prevProps.query.fields?.join(', ') || '';
    if (currentFields !== prevFields && currentFields !== this.state.fieldsInput) {
      this.setState({ fieldsInput: currentFields });
    }
  }

  queryTypeOptions = [
    { label: 'Collection Query', value: 'collection' },
    { label: 'Custom AQL', value: 'aql' },
  ];

  aggregationOptions = [
    { label: 'None', value: '' },
    { label: 'COUNT', value: 'COUNT' },
    { label: 'SUM', value: 'SUM' },
    { label: 'AVG', value: 'AVG' },
    { label: 'MIN', value: 'MIN' },
    { label: 'MAX', value: 'MAX' },
  ];

  sortOrderOptions = [
    { label: 'ASC', value: 'ASC' },
    { label: 'DESC', value: 'DESC' },
  ];

  onQueryChange = (value: Partial<ArangoDBQuery>) => {
    const { onChange, query, onRunQuery } = this.props;
    const newQuery = { ...query, ...value };
    onChange(newQuery);
    
    // Add debug logging
    console.log('ArangoDB Query Changed:', newQuery);
    
    // Trigger query execution for certain changes
    if (value.aqlQuery !== undefined || value.collection !== undefined) {
      console.log('Triggering query execution...');
      onRunQuery();
    }
  };

  onQueryTypeChange = (value: string) => {
    this.onQueryChange({ queryType: value as 'aql' | 'collection' });
  };

  onCollectionChange = (event: ChangeEvent<HTMLInputElement>) => {
    this.onQueryChange({ collection: event.target.value });
  };

  onAQLQueryChange = (event: ChangeEvent<HTMLTextAreaElement>) => {
    this.onQueryChange({ aqlQuery: event.target.value });
  };

  onFilterChange = (event: ChangeEvent<HTMLInputElement>) => {
    this.onQueryChange({ filter: event.target.value });
  };

  onLimitChange = (event: ChangeEvent<HTMLInputElement>) => {
    const value = event.target.value;
    let limit: number | undefined;
    
    if (value === '' || value === '0') {
      // Empty or zero means no limit (or use default)
      limit = undefined;
    } else {
      const parsed = parseInt(value, 10);
      limit = isNaN(parsed) ? undefined : parsed;
    }
    
    console.log('Limit changed:', value, '-> parsed as:', limit);
    this.onQueryChange({ limit });
  };

  onTimeFieldChange = (event: ChangeEvent<HTMLInputElement>) => {
    this.onQueryChange({ timeField: event.target.value });
  };

  onValueFieldChange = (event: ChangeEvent<HTMLInputElement>) => {
    this.onQueryChange({ valueField: event.target.value });
  };

  onGroupByChange = (event: ChangeEvent<HTMLInputElement>) => {
    this.onQueryChange({ groupBy: event.target.value });
  };

  onAggregationChange = (value: SelectableValue<string>) => {
    this.onQueryChange({ aggregation: value.value as any });
  };

  onSortByChange = (event: ChangeEvent<HTMLInputElement>) => {
    this.onQueryChange({ sortBy: event.target.value });
  };

  onSortOrderChange = (value: SelectableValue<string>) => {
    this.onQueryChange({ sortOrder: value.value as 'ASC' | 'DESC' });
  };

  onFieldsChange = (event: ChangeEvent<HTMLInputElement>) => {
    const value = event.target.value;
    // Update local state to preserve user input exactly as typed
    this.setState({ fieldsInput: value });
    
    // Process the fields - split by comma and clean up for the query
    const fields = value === '' ? [] : value.split(',').map(f => f.trim()).filter(f => f !== '');
    this.onQueryChange({ fields });
  };

  onRunQueryClick = () => {
    console.log('Manual run query clicked');
    this.props.onRunQuery();
  };

  render() {
    const { query } = this.props;

    return (
      <div>
        <InlineFieldRow>
          <InlineField label="Query Type" labelWidth={12}>
            <RadioButtonGroup
              options={this.queryTypeOptions}
              value={query.queryType || 'collection'}
              onChange={this.onQueryTypeChange}
            />
          </InlineField>
          <Button onClick={this.onRunQueryClick} variant="primary">
            Run Query
          </Button>
        </InlineFieldRow>

        {query.queryType === 'aql' ? (
          <div>
            <InlineFieldRow>
              <InlineField label="AQL Query" labelWidth={12} grow>
                <TextArea
                  rows={8}
                  value={query.aqlQuery || ''}
                  onChange={this.onAQLQueryChange}
                  placeholder={`FOR doc IN my_collection
  FILTER doc.timestamp >= $__timeFrom AND doc.timestamp <= $__timeTo
  SORT doc.timestamp ASC
  RETURN doc`}
                />
              </InlineField>
            </InlineFieldRow>

            <div style={{ marginTop: '8px', fontSize: '12px', color: '#999' }}>
              <strong>Available variables:</strong>
              <br />
              • <code>$__timeFrom</code> - Start of time range (timestamp)
              <br />
              • <code>$__timeTo</code> - End of time range (timestamp)
              <br />
              • <code>$__timeFilter(field)</code> - Expands to: field {'>'}= $__timeFrom AND field {'<'}= $__timeTo
            </div>
          </div>
        ) : (
          <div>
            <InlineFieldRow>
              <InlineField label="Collection" labelWidth={12}>
                <Input
                  width={30}
                  value={query.collection || ''}
                  onChange={this.onCollectionChange}
                  placeholder="Enter collection name"
                />
              </InlineField>
            </InlineFieldRow>

            {query.collection && (
              <>
                <InlineFieldRow>
                  <InlineField label="Time Field" labelWidth={12}>
                    <Input
                      width={30}
                      value={query.timeField || ''}
                      onChange={this.onTimeFieldChange}
                      placeholder="timestamp (optional)"
                    />
                  </InlineField>
                  
                  <InlineField label="Value Field" labelWidth={12}>
                    <Input
                      width={30}
                      value={query.valueField || ''}
                      onChange={this.onValueFieldChange}
                      placeholder="field name"
                    />
                  </InlineField>
                </InlineFieldRow>

                <InlineFieldRow>
                  <InlineField label="Filter" labelWidth={12} grow>
                    <Input
                      value={query.filter || ''}
                      onChange={this.onFilterChange}
                      placeholder="status == 'active' AND value > 100"
                    />
                  </InlineField>
                </InlineFieldRow>

                <InlineFieldRow>
                  <InlineField label="Group By" labelWidth={12}>
                    <Input
                      width={30}
                      value={query.groupBy || ''}
                      onChange={this.onGroupByChange}
                      placeholder="field name (optional)"
                    />
                  </InlineField>
                  
                  <InlineField label="Aggregation" labelWidth={12}>
                    <Select
                      width={30}
                      options={this.aggregationOptions}
                      value={this.aggregationOptions.find(a => a.value === query.aggregation)}
                      onChange={this.onAggregationChange}
                      placeholder="None"
                    />
                  </InlineField>
                </InlineFieldRow>

                <InlineFieldRow>
                  <InlineField label="Sort By" labelWidth={12}>
                    <Input
                      width={30}
                      value={query.sortBy || ''}
                      onChange={this.onSortByChange}
                      placeholder="field name (optional)"
                    />
                  </InlineField>
                  
                  <InlineField label="Sort Order" labelWidth={12}>
                    <Select
                      width={30}
                      options={this.sortOrderOptions}
                      value={this.sortOrderOptions.find(s => s.value === query.sortOrder)}
                      onChange={this.onSortOrderChange}
                    />
                  </InlineField>
                  
                  <InlineField label="Limit" labelWidth={12}>
                    <Input
                      width={30}
                      type="number"
                      value={query.limit || ''}
                      onChange={this.onLimitChange}
                      placeholder="1000"
                    />
                  </InlineField>
                </InlineFieldRow>

                <InlineFieldRow>
                  <InlineField label="Return Fields" labelWidth={12} grow>
                    <Input
                      value={this.state.fieldsInput}
                      onChange={this.onFieldsChange}
                      placeholder="field1, field2, field3 (leave empty for all fields)"
                    />
                  </InlineField>
                </InlineFieldRow>
              </>
            )}
          </div>
        )}
      </div>
    );
  }
}
