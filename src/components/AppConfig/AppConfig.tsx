import React, { ChangeEvent, useState } from 'react';
import { lastValueFrom } from 'rxjs';
import { css } from '@emotion/css';
import { AppPluginMeta, GrafanaTheme2, PluginConfigPageProps, PluginMeta } from '@grafana/data';
import { getBackendSrv } from '@grafana/runtime';
import { Button, Field, FieldSet, SecretInput, useStyles2 } from '@grafana/ui';
import { testIds } from '../testIds';

type AppPluginSettings = {
  postgresDsn?: string;
};

type State = {
  // Tells us if the API key secret is set.
  isApiKeySet: boolean;
  postgresDsn: string;
};

export interface AppConfigProps extends PluginConfigPageProps<AppPluginMeta<AppPluginSettings>> {}

const AppConfig = ({ plugin }: AppConfigProps) => {
  const s = useStyles2(getStyles);
  const { enabled, pinned, jsonData, secureJsonFields } = plugin.meta;
  const [state, setState] = useState<State>({
    isApiKeySet: Boolean(secureJsonFields?.apiKey),
    // New: load DSN from jsonData
    postgresDsn: (jsonData as any)?.postgresDsn || '',
  } as State & { postgresDsn: string });

  const isSubmitDisabled = Boolean((!state.isApiKeySet && !state.postgresDsn));

  const onResetApiKey = () =>
    setState({
      ...state,
      postgresDsn: '',
      isApiKeySet: false,
    });

  const onChange = (event: ChangeEvent<HTMLInputElement>) => {
    setState({
      ...state,
      [event.target.name]: event.target.value.trim(),
    });
  };

  const onSubmit = () => {
    if (isSubmitDisabled) {
      return;
    }

    updatePluginAndReload(plugin.meta.id, {
      enabled,
      pinned,
      jsonData: {
      },
      // This cannot be queried later by the frontend.
      // We don't want to override it in case it was set previously and left untouched now.
      secureJsonData: state.isApiKeySet
        ? undefined
        : {
            postgresDsn: (state as any).postgresDsn,
          },
    });
  };

  return (
    <form onSubmit={onSubmit}>
      <FieldSet label="API Settings">
        <Field
          label="Postgres DSN"
          description="Connection string used by the backend to connect to Postgres"
          className={s.marginTop}
        >
          <SecretInput
            width={60}
            name="postgresDsn"
            id="config-postgres-dsn"
            value={(state as any).postgresDsn}
            isConfigured={state.isApiKeySet}
            placeholder={`E.g.: host=localhost user=forge password=secret dbname=forge sslmode=disable`}
            onChange={onChange}
            onReset={onResetApiKey}
          />
        </Field>

        <div className={s.marginTop}>
          <Button type="submit" data-testid={testIds.appConfig.submit} disabled={isSubmitDisabled}>
            Save
          </Button>
        </div>
      </FieldSet>
    </form>
  );
};

export default AppConfig;

const getStyles = (theme: GrafanaTheme2) => ({
  colorWeak: css`
    color: ${theme.colors.text.secondary};
  `,
  marginTop: css`
    margin-top: ${theme.spacing(3)};
  `,
});

const updatePluginAndReload = async (pluginId: string, data: Partial<PluginMeta<AppPluginSettings>>) => {
  try {
    await updatePlugin(pluginId, data);

    // Reloading the page as the changes made here wouldn't be propagated to the actual plugin otherwise.
    // This is not ideal, however unfortunately currently there is no supported way for updating the plugin state.
    window.location.reload();
  } catch (e) {
    console.error('Error while updating the plugin', e);
  }
};

const updatePlugin = async (pluginId: string, data: Partial<PluginMeta>) => {
  const response = await getBackendSrv().fetch({
    url: `/api/plugins/${pluginId}/settings`,
    method: 'POST',
    data,
  });

  return lastValueFrom(response);
};
