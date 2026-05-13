import { DataSourceJsonData } from '@grafana/data';
import { DataQuery } from '@grafana/schema';

export type QueryType = 'time_series' | 'table' | 'csv';

export interface MyQuery extends DataQuery {
  queryText?: string;
  apiEndpoint?: string;
  queryType?: QueryType;
  device?: string;
  child?: string;
  attribute?: string;
  omitParents?: boolean;
}

export const DEFAULT_QUERY: Partial<MyQuery> = {
  queryText:
    'series interval avg ${__timeInterval} time "from ${__timeFrom} to ${__timeTo}" * "${__device}" "${__child}" "${__attribute}"',
  apiEndpoint: 'api-db',
  queryType: 'time_series',
};

export interface MyDataSourceOptions extends DataSourceJsonData {
  url?: string;
  skipTLS?: boolean;
  username?: string;
  defaultDevice?: string;
  defaultChild?: string;
  defaultAttribute?: string;
}

export interface MySecureJsonData {
  apiKey?: string;
  password?: string;
}
