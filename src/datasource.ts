import { DataQueryRequest, DataSourceInstanceSettings, CoreApp, ScopedVars, MetricFindValue, toDataFrame } from '@grafana/data';
import { DataSourceWithBackend, getTemplateSrv } from '@grafana/runtime';
import { lastValueFrom } from 'rxjs';

import { MyQuery, MyDataSourceOptions, DEFAULT_QUERY } from './types';

export class DataSource extends DataSourceWithBackend<MyQuery, MyDataSourceOptions> {
  jsonData: MyDataSourceOptions;

  constructor(instanceSettings: DataSourceInstanceSettings<MyDataSourceOptions>) {
    super(instanceSettings);
    this.jsonData = instanceSettings.jsonData;
  }

  getDefaultQuery(_: CoreApp): Partial<MyQuery> {
    return {
      ...DEFAULT_QUERY,
      device: this.jsonData.defaultDevice,
      child: this.jsonData.defaultChild,
      attribute: this.jsonData.defaultAttribute,
    };
  }

  applyTemplateVariables(query: MyQuery, scopedVars: ScopedVars): MyQuery {
    const srv = getTemplateSrv();
    return {
      ...query,
      queryText: srv.replace(query.queryText, scopedVars),
      apiEndpoint: srv.replace(query.apiEndpoint, scopedVars),
      device: srv.replace(query.device, scopedVars),
      child: srv.replace(query.child, scopedVars),
      attribute: srv.replace(query.attribute, scopedVars),
    };
  }

  filterQuery(query: MyQuery): boolean {
    return !!query.queryText;
  }

  async listDevices(): Promise<string[]> {
    const res = await this.getResource('devices');
    return res?.values ?? [];
  }

  async listChildren(device: string): Promise<Array<{ value: string; label: string }>> {
    const res = await this.getResource('children', { device });
    return res?.items ?? [];
  }

  async listAttributes(device: string, child: string): Promise<string[]> {
    const res = await this.getResource('attributes', { device, child });
    return res?.values ?? [];
  }

  // metricFindQuery powers dashboard variable dropdowns.
  // It runs a table query with omitParents=true and returns the first string column.
  async metricFindQuery(request: string): Promise<MetricFindValue[]> {
    const targets: MyQuery[] = [
      {
        refId: 'variable',
        queryType: 'table',
        queryText: getTemplateSrv().replace(request, {}),
        apiEndpoint: 'api-db',
        omitParents: true,
      },
    ];

    const response = await lastValueFrom(this.query({ targets } as DataQueryRequest<MyQuery>));
    if (response?.data?.length) {
      const df = toDataFrame(response.data[0]);
      if (df.fields.length && df.fields[0].type === 'string') {
        return (df.fields[0].values as string[]).map((v) => ({ text: v }));
      }
    }
    return [];
  }
}
