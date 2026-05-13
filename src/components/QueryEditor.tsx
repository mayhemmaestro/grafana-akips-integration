import React, { ChangeEvent, useCallback } from 'react';
import { Combobox, InlineField, Input, RadioButtonGroup, Stack, Switch } from '@grafana/ui';
import { QueryEditorProps, SelectableValue } from '@grafana/data';
import { DataSource } from '../datasource';
import { MyDataSourceOptions, MyQuery, QueryType } from '../types';

type Props = QueryEditorProps<DataSource, MyQuery, MyDataSourceOptions>;

const QUERY_TYPES: Array<SelectableValue<QueryType>> = [
  { label: 'Time series', value: 'time_series' },
  { label: 'Table', value: 'table' },
  { label: 'CSV', value: 'csv' },
];

export function QueryEditor({ query, onChange, onRunQuery, datasource }: Props) {
  const { queryText, apiEndpoint, queryType, device, child, attribute, omitParents } = query;

  const onQueryTextChange = (event: ChangeEvent<HTMLInputElement>) => {
    onChange({ ...query, queryText: event.target.value });
  };

  const onEndpointChange = (event: ChangeEvent<HTMLInputElement>) => {
    onChange({ ...query, apiEndpoint: event.target.value });
  };

  const onQueryTypeChange = (value: QueryType) => {
    onChange({ ...query, queryType: value });
  };

  const onDeviceChange = (value: string) => {
    onChange({ ...query, device: value, child: '', attribute: '' });
    onRunQuery();
  };

  const onChildChange = (value: string) => {
    onChange({ ...query, child: value, attribute: '' });
    onRunQuery();
  };

  const onAttributeChange = (value: string) => {
    onChange({ ...query, attribute: value });
    onRunQuery();
  };

  const onOmitParentsChange = (event: React.FormEvent<HTMLInputElement>) => {
    onChange({ ...query, omitParents: event.currentTarget.checked });
    onRunQuery();
  };

  const loadEndpointOptions = useCallback(async (q: string) => {
    const options = [
      { label: 'api-db', value: 'api-db' },
      { label: 'api-msg', value: 'api-msg' },
    ];
    return options.filter((o) => o.label.toLowerCase().includes(q.toLowerCase()));
  }, []);

  const loadDevices = useCallback(
    async (q: string) => {
      const values = await datasource.listDevices();
      return values
        .filter((v) => v.toLowerCase().includes(q.toLowerCase()))
        .map((v) => ({ label: v, value: v }));
    },
    [datasource]
  );

  const loadChildren = useCallback(
    async (q: string) => {
      const items = await datasource.listChildren(device || '*');
      return items
        .filter(
          (item) =>
            item.label.toLowerCase().includes(q.toLowerCase()) ||
            item.value.toLowerCase().includes(q.toLowerCase())
        )
        .map((item) => ({ label: item.label, value: item.value }));
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [datasource, device]
  );

  const loadAttributes = useCallback(
    async (q: string) => {
      const values = await datasource.listAttributes(device || '*', child || '*');
      return values
        .filter((v) => v.toLowerCase().includes(q.toLowerCase()))
        .map((v) => ({ label: v, value: v }));
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [datasource, device, child]
  );

  return (
    <Stack direction="column" gap={0}>
      <Stack gap={0}>
        <InlineField label="API Endpoint">
          <Combobox
            onChange={(v) => onEndpointChange({ target: { value: v.value } } as ChangeEvent<HTMLInputElement>)}
            options={loadEndpointOptions}
            value={apiEndpoint || 'api-db'}
          />
        </InlineField>

        <InlineField label="Query type" labelWidth={12}>
          <RadioButtonGroup<QueryType>
            options={QUERY_TYPES}
            value={queryType ?? 'time_series'}
            onChange={onQueryTypeChange}
          />
        </InlineField>

        {queryType === 'table' && (
          <InlineField label="Omit parents" labelWidth={14} style={{ alignItems: 'center' }}>
            <Switch value={omitParents ?? false} onChange={onOmitParentsChange} />
          </InlineField>
        )}
      </Stack>

      <Stack gap={0}>
        <InlineField label="Device" labelWidth={10} tooltip="Sets $__device in the query">
          <Combobox
            onChange={(v) => onDeviceChange(v?.value ?? '')}
            options={loadDevices}
            value={device || ''}
            placeholder="e.g. my-router-01"
            width={24}
            createCustomValue
          />
        </InlineField>

        <InlineField label="Child" labelWidth={8} tooltip="Sets $__child in the query">
          <Combobox
            onChange={(v) => onChildChange(v?.value ?? '')}
            options={loadChildren}
            value={child || ''}
            placeholder="e.g. eth0"
            width={24}
            createCustomValue
          />
        </InlineField>

        <InlineField label="Attribute" labelWidth={12} tooltip="Sets $__attribute in the query">
          <Combobox
            onChange={(v) => onAttributeChange(v?.value ?? '')}
            options={loadAttributes}
            value={attribute || ''}
            placeholder="e.g. ifInOctets"
            width={24}
            createCustomValue
          />
        </InlineField>
      </Stack>

      <InlineField
        label="AKIPS Query"
        labelWidth={16}
        grow
        tooltip='AKIPS command, e.g. series interval avg $__timeInterval time "from $__timeFrom to $__timeTo" * "$__device" "$__child" "$__attribute"'
      >
        <Input
          id="query-editor-query-text"
          onChange={onQueryTextChange}
          value={queryText || ''}
          placeholder='series interval avg $__timeInterval time "from $__timeFrom to $__timeTo" * "$__device" "$__child" "$__attribute"'
          onBlur={onRunQuery}
        />
      </InlineField>
    </Stack>
  );
}
