import React from 'react';
import { render, screen } from '@testing-library/react';
import { PluginType } from '@grafana/data';
import AppConfig, { AppConfigProps } from './AppConfig';
import { testIds } from 'components/testIds';

describe('Components/AppConfig', () => {
  let props: AppConfigProps;

  beforeEach(() => {
    jest.resetAllMocks();

    props = {
      plugin: {
        meta: {
          id: 'sample-app',
          name: 'Sample App',
          type: PluginType.app,
          enabled: true,
          jsonData: {},
        },
      },
      query: {},
    } as unknown as AppConfigProps;
  });

  test('renders Forge settings with Postgres/Redis secrets and LGTM URL inputs', () => {
    const plugin = { meta: { ...props.plugin.meta, enabled: false } };

    // @ts-ignore - We don't need to provide `addConfigPage()` and `setChannelSupport()` for these tests
    render(<AppConfig plugin={plugin} query={props.query} />);

    expect(screen.getByRole('group', { name: /forge settings/i })).toBeInTheDocument();
    expect(
      screen.getByPlaceholderText(/host=localhost user=forge password=secret dbname=forge sslmode=disable/i)
    ).toBeInTheDocument();
    expect(screen.getByPlaceholderText(/redis:\/\/:password@localhost:6379\/0/i)).toBeInTheDocument();
    expect(screen.getByPlaceholderText(/http:\/\/localhost:9009\/api\/v1\/push/i)).toBeInTheDocument();
    expect(screen.getByPlaceholderText(/http:\/\/localhost:3100\/loki\/api\/v1\/push/i)).toBeInTheDocument();
    expect(screen.getByPlaceholderText(/http:\/\/localhost:4318/i)).toBeInTheDocument();
    expect(screen.getByTestId(testIds.appConfig.submit)).toBeDisabled();
  });
});
