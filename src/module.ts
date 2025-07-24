import { DataSourcePlugin } from '@grafana/data';
import { DataSource } from './datasource';
import { ConfigEditor } from './ConfigEditor';
import { QueryEditor } from './QueryEditor';
import { ArangoDBQuery, ArangoDBDataSourceOptions } from './types';

export const plugin = new DataSourcePlugin<DataSource, ArangoDBQuery, ArangoDBDataSourceOptions>(DataSource as any)
  .setConfigEditor(ConfigEditor as any)
  .setQueryEditor(QueryEditor as any);
