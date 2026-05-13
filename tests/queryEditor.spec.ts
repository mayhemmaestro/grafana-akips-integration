import { test, expect } from '@grafana/plugin-e2e';

test('smoke: should render query editor controls', async ({ panelEditPage, readProvisionedDataSource }) => {
  const ds = await readProvisionedDataSource({ fileName: 'datasources.yml' });
  await panelEditPage.datasource.set(ds.name);

  const row = panelEditPage.getQueryEditorRow('A');
  // InlineField label text is visible
  await expect(row.getByText('API Endpoint', { exact: true })).toBeVisible();
  // AKIPS Query input is a standard Input with id that gets associated to its InlineField label
  await expect(row.getByRole('textbox', { name: 'AKIPS Query' })).toBeVisible();
});

test('should render query type options', async ({ panelEditPage, readProvisionedDataSource }) => {
  const ds = await readProvisionedDataSource({ fileName: 'datasources.yml' });
  await panelEditPage.datasource.set(ds.name);

  const row = panelEditPage.getQueryEditorRow('A');
  await expect(row.getByRole('radio', { name: 'Time series' })).toBeVisible();
  await expect(row.getByRole('radio', { name: 'Table' })).toBeVisible();
  await expect(row.getByRole('radio', { name: 'CSV' })).toBeVisible();
});

test('should render device, child, and attribute selectors', async ({ panelEditPage, readProvisionedDataSource }) => {
  const ds = await readProvisionedDataSource({ fileName: 'datasources.yml' });
  await panelEditPage.datasource.set(ds.name);

  const row = panelEditPage.getQueryEditorRow('A');
  // Combobox accessible names come from placeholder when no value is set
  await expect(row.getByPlaceholder('e.g. my-router-01')).toBeVisible();
  await expect(row.getByPlaceholder('e.g. eth0')).toBeVisible();
  await expect(row.getByPlaceholder('e.g. ifInOctets')).toBeVisible();
});
