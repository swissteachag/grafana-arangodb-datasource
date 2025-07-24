import {
  DataSourceInstanceSettings,
  CoreApp,
  ScopedVars,
} from '@grafana/data';
import { DataSourceWithBackend, getTemplateSrv } from '@grafana/runtime';
import { ArangoDBQuery, ArangoDBDataSourceOptions } from './types';

export class DataSource extends DataSourceWithBackend<ArangoDBQuery, ArangoDBDataSourceOptions> {
  constructor(instanceSettings: DataSourceInstanceSettings<ArangoDBDataSourceOptions>) {
    super(instanceSettings);
  }

  /**
   * Apply template variables to query
   */
  applyTemplateVariables(query: ArangoDBQuery, scopedVars: ScopedVars): ArangoDBQuery {
    return {
      ...query,
      aqlQuery: query.aqlQuery ? getTemplateSrv().replace(query.aqlQuery, scopedVars) : '',
      collection: getTemplateSrv().replace(query.collection || '', scopedVars),
    };
  }

  /**
   * Provide default query for new queries
   */
  getDefaultQuery(_: CoreApp): Partial<ArangoDBQuery> {
    return {
      queryType: 'collection',
      collection: '',
      aqlQuery: '',
      timeField: '',
      valueField: '',
      limit: 1000,
    };
  }

  /**
   * Filter out queries that shouldn't be executed
   */
  filterQuery(query: ArangoDBQuery): boolean {
    if (query.queryType === 'aql') {
      return !!query.aqlQuery;
    }
    return !!query.collection;
  }
}
