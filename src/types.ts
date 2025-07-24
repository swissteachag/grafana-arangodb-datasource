import { DataQuery, DataSourceJsonData } from '@grafana/data';

export interface ArangoDBQuery extends DataQuery {
  // Query type: 'aql' for custom AQL queries, 'collection' for collection queries
  queryType: 'aql' | 'collection';
  
  // Reference ID for the query
  refId: string;
  
  // Custom AQL query
  aqlQuery?: string;
  
  // Collection-based query options
  collection?: string;
  filter?: string;
  limit?: number;
  sortBy?: string;
  sortOrder?: 'ASC' | 'DESC';
  
  // Time field for time series data
  timeField?: string;
  
  // Value field for metrics
  valueField?: string;
  
  // Additional fields to return
  fields?: string[];
  
  // Group by field
  groupBy?: string;
  
  // Aggregation function
  aggregation?: 'COUNT' | 'SUM' | 'AVG' | 'MIN' | 'MAX';
}

export interface ArangoDBDataSourceOptions extends DataSourceJsonData {
  // Connection settings
  url: string;
  database: string;
  
  // Authentication
  username?: string;
  
  // Collection discovery
  collections?: string[];
  
  // Connection options
  timeout?: number;
  maxConnections?: number;
  
  // Query performance settings
  batchSize?: number; // Maximum number of documents per batch (default: 50000)
}

export interface ArangoDBSecureJsonData {
  password?: string;
}

export interface CollectionInfo {
  name: string;
  type: 'document' | 'edge';
  count: number;
}

export interface QueryResult {
  columns: string[];
  rows: any[][];
  meta?: {
    executionTime: number;
    scannedDocuments: number;
    filteredDocuments: number;
  };
}
