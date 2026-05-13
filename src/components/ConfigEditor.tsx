import React, { ChangeEvent } from 'react';
import { InlineField, Input, SecretInput, Switch } from '@grafana/ui';
import { DataSourcePluginOptionsEditorProps } from '@grafana/data';
import { MyDataSourceOptions, MySecureJsonData } from '../types';

interface Props extends DataSourcePluginOptionsEditorProps<MyDataSourceOptions, MySecureJsonData> {}

export function ConfigEditor(props: Props) {
  const { onOptionsChange, options } = props;
  const { jsonData, secureJsonFields, secureJsonData } = options;

  const onPathChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({
      ...options,
      jsonData: {
        ...jsonData,
        url: event.target.value,
      },
    });
  };

  const onResetPassword = () => {
    onOptionsChange({
      ...options,
      secureJsonData: {
        ...options.secureJsonData,
        password: '',
      },
    });
  };

  const onUsernameChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({
      ...options,
      jsonData: {
        ...jsonData,
        username: event.target.value,
      },
    });
  };

  const onPasswordChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({
      ...options,
      secureJsonData: {
        ...secureJsonData,
        password: event.target.value,
      },
    });
  };

  return (
    <>
      <InlineField
        label="URL"
        labelWidth={14}
        interactive
        tooltip={'URL for AKIPS instance, e.g. https://my-akips-instance.com'}
      >
        <Input
          id="config-editor-path"
          onChange={onPathChange}
          value={jsonData.url}
          placeholder="https://my-akips-instance.com"
          width={40}
        />
      </InlineField>
      {/* Username Field */}
      <InlineField label="Username" labelWidth={14}>
        <Input
          id="config-editor-username"
          value={jsonData.username || ''}
          placeholder="api-ro or api-rw"
          width={40}
          onChange={onUsernameChange}
        />
      </InlineField>

      {/* Password Field */}
      <InlineField
        label="Password"
        labelWidth={14}
        interactive
        tooltip={'Secure json field (backend only)'}
      >
        <SecretInput
          required
          id="config-editor-password"
          isConfigured={secureJsonFields.password}
          value={secureJsonData?.password || ''}
          placeholder="Enter your password"
          width={40}
          onReset={onResetPassword}
          onChange={onPasswordChange}
        />
      </InlineField>
      <InlineField
        label="Skip TLS"
        labelWidth={14}
        style={{ alignItems: 'center' }}
        tooltip="Performs an insecure TLS connection by skipping certificate validation."
      >
        <Switch
          id="config-editor-skipTLS-bool"
          value={jsonData.skipTLS || false}
          onChange={(event) => {
            onOptionsChange({
              ...options,
              jsonData: {
                ...jsonData,
                skipTLS: event.currentTarget.checked,
              },
            });
          }}
        />
      </InlineField>
    </>
  );
}
