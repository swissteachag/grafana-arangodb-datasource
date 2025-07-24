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
   * Validate that AQL query doesn't contain write operations
   */
  private validateReadOnlyQuery(query: string): boolean {
    if (!query) {
      return true;
    }

    // Convert query to uppercase for case-insensitive matching
    const upperQuery = query.toUpperCase();
    
    // List of write operations that should be blocked
    const writeOperations = ['REMOVE', 'UPDATE', 'REPLACE', 'INSERT', 'UPSERT'];
    
    // Check for each write operation
    for (const operation of writeOperations) {
      // Use regex to match the operation as a whole word (not part of another word)
      const pattern = new RegExp(`\\b${operation}\\b`);
      if (pattern.test(upperQuery)) {
        return false;
      }
    }
    
    return true;
  }

  /**
   * Filter out queries that shouldn't be executed
   */
  filterQuery(query: ArangoDBQuery): boolean {
    if (query.queryType === 'aql') {
      // Check if query exists and validate it doesn't contain write operations
      return !!query.aqlQuery && this.validateReadOnlyQuery(query.aqlQuery);
    }
    return !!query.collection;
  }
}
