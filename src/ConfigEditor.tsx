import React, { PureComponent, ChangeEvent } from 'react';
import {
  DataSourcePluginOptionsEditorProps,
  onUpdateDatasourceJsonDataOptionSelect,
  onUpdateDatasourceJsonDataOption,
  onUpdateDatasourceSecureJsonDataOption,
  updateDatasourcePluginResetOption,
} from '@grafana/data';
import {
  Field,
  Input,
  Button,
  Legend,
  InlineField,
  InlineSwitch,
  Select,
  TextArea,
  InlineFieldRow,
} from '@grafana/ui';
import { ArangoDBDataSourceOptions, ArangoDBSecureJsonData, CollectionInfo } from './types';

interface Props extends DataSourcePluginOptionsEditorProps<ArangoDBDataSourceOptions, ArangoDBSecureJsonData> {}

interface State {
  collections: CollectionInfo[];
}

export class ConfigEditor extends PureComponent<Props, State> {
  state: State = {
    collections: [],
  };

  componentDidMount() {
    this.loadCollections();
  }

  onURLChange = (event: ChangeEvent<HTMLInputElement>) => {
    const { onOptionsChange, options } = this.props;
    const jsonData = {
      ...options.jsonData,
      url: event.target.value,
    };
    onOptionsChange({ ...options, jsonData });
  };

  onDatabaseChange = (event: ChangeEvent<HTMLInputElement>) => {
    const { onOptionsChange, options } = this.props;
    const jsonData = {
      ...options.jsonData,
      database: event.target.value,
    };
    onOptionsChange({ ...options, jsonData });
  };

  onUsernameChange = (event: ChangeEvent<HTMLInputElement>) => {
    const { onOptionsChange, options } = this.props;
    const jsonData = {
      ...options.jsonData,
      username: event.target.value,
    };
    onOptionsChange({ ...options, jsonData });
  };

  onPasswordChange = (event: ChangeEvent<HTMLInputElement>) => {
    const { onOptionsChange, options } = this.props;
    onOptionsChange({
      ...options,
      secureJsonData: {
        ...options.secureJsonData,
        password: event.target.value,
      },
    });
  };

  onTimeoutChange = (event: ChangeEvent<HTMLInputElement>) => {
    const { onOptionsChange, options } = this.props;
    const jsonData = {
      ...options.jsonData,
      timeout: parseInt(event.target.value, 10) || 30,
    };
    onOptionsChange({ ...options, jsonData });
  };

  onBatchSizeChange = (event: ChangeEvent<HTMLInputElement>) => {
    const { onOptionsChange, options } = this.props;
    const jsonData = {
      ...options.jsonData,
      batchSize: parseInt(event.target.value, 10) || 50000,
    };
    onOptionsChange({ ...options, jsonData });
  };

  onResetPassword = () => {
    const { onOptionsChange, options } = this.props;
    onOptionsChange({
      ...options,
      secureJsonFields: {
        ...options.secureJsonFields,
        password: false,
      },
      secureJsonData: {
        ...options.secureJsonData,
        password: '',
      },
    });
  };

  loadCollections = async () => {
    try {
      // This would load collections from ArangoDB
      // For now, we'll just initialize empty
      this.setState({ collections: [] });
    } catch (error) {
      console.error('Failed to load collections:', error);
    }
  };

  render() {
    const { onOptionsChange, options } = this.props;
    const { jsonData, secureJsonFields, secureJsonData } = options;
    const { collections } = this.state;

    return (
      <div className="gf-form-group">
        <h3 className="page-heading">ArangoDB Connection</h3>
        
        <div className="gf-form-group">
          <InlineField label="URL" labelWidth={12} required>
            <Input
              width={40}
              value={jsonData.url || ''}
              placeholder="http://localhost:8529"
              onChange={this.onURLChange}
            />
          </InlineField>
          
          <InlineField label="Database" labelWidth={12}>
            <Input
              width={40}
              value={jsonData.database || '_system'}
              placeholder="_system"
              onChange={this.onDatabaseChange}
            />
          </InlineField>
        </div>

        <div className="gf-form-group">
          <Legend>Authentication</Legend>
          
          <InlineField label="Username" labelWidth={12}>
            <Input
              width={40}
              value={jsonData.username || ''}
              placeholder="Username (optional)"
              onChange={this.onUsernameChange}
            />
          </InlineField>
          
          <InlineField label="Password" labelWidth={12}>
            <Input
              width={40}
              type="password"
              value={secureJsonData?.password || ''}
              placeholder={secureJsonFields?.password ? 'Password configured' : 'Password (optional)'}
              onChange={this.onPasswordChange}
            />
          </InlineField>
          
          {secureJsonFields?.password && (
            <div className="gf-form-inline">
              <div className="gf-form">
                <Button variant="secondary" type="button" onClick={this.onResetPassword}>
                  Reset password
                </Button>
              </div>
            </div>
          )}
        </div>

        <div className="gf-form-group">
          <Legend>Connection Options</Legend>
          
          <InlineField label="Timeout (seconds)" labelWidth={12}>
            <Input
              width={40}
              type="number"
              value={jsonData.timeout || 30}
              min={1}
              max={300}
              onChange={this.onTimeoutChange}
            />
          </InlineField>

          <InlineField 
            label="Batch Size" 
            labelWidth={12}
            tooltip="Maximum number of documents retrieved per batch. Higher values improve performance for large result sets but use more memory."
          >
            <Input
              width={40}
              type="number"
              value={jsonData.batchSize || 50000}
              min={1000}
              max={1000000}
              placeholder="50000"
              onChange={this.onBatchSizeChange}
            />
          </InlineField>
        </div>

        {collections.length > 0 && (
          <div className="gf-form-group">
            <Legend>Available Collections</Legend>
            <div className="gf-form">
              <div className="gf-form-label width-10">Collections found:</div>
              <div className="gf-form-label">
                {collections.map(col => col.name).join(', ')}
              </div>
            </div>
          </div>
        )}
      </div>
    );
  }
}
