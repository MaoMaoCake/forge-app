import React, { ChangeEvent, useState } from 'react';
import { lastValueFrom } from 'rxjs';
import { css } from '@emotion/css';
import { AppPluginMeta, GrafanaTheme2, PluginConfigPageProps, PluginMeta } from '@grafana/data';
import { getBackendSrv } from '@grafana/runtime';
import { Button, Field, FieldSet, Input, SecretInput, useStyles2 } from '@grafana/ui';
import { testIds } from '../testIds';

type AppPluginSettings = {
  postgresDsn?: string;
  redis_url?: string;
  mimir_url?: string;
  loki_url?: string;
  tempo_url?: string;
};

type State = {
  postgresDsnSet: boolean;
  redis_url_set: boolean;
  postgresDsn: string;
  redis_url: string;
  mimir_url: string;
  loki_url: string;
  tempo_url: string;
};

export interface AppConfigProps extends PluginConfigPageProps<AppPluginMeta<AppPluginSettings>> {}

const AppConfig = ({ plugin }: AppConfigProps) => {
  const s = useStyles2(getStyles);
  const { enabled, pinned, jsonData, secureJsonFields } = plugin.meta;
  const pluginJsonData = (jsonData ?? {}) as AppPluginSettings;

  const [state, setState] = useState<State>({
    postgresDsnSet: Boolean(secureJsonFields?.postgresDsn),
    redis_url_set: Boolean(secureJsonFields?.redis_url),
    postgresDsn: secureJsonFields?.postgresDsn ? '' : pluginJsonData.postgresDsn || '',
    redis_url: secureJsonFields?.redis_url ? '' : pluginJsonData.redis_url || '',
    mimir_url: pluginJsonData.mimir_url || '',
    loki_url: pluginJsonData.loki_url || '',
    tempo_url: pluginJsonData.tempo_url || '',
  });

  const isSubmitDisabled =
    (!state.postgresDsnSet && !state.postgresDsn) || (!state.redis_url_set && !state.redis_url);

  const onResetPostgresDsn = () =>
    setState({
      ...state,
      postgresDsn: '',
      postgresDsnSet: false,
    });

  const onResetRedisUrl = () =>
    setState({
      ...state,
      redis_url: '',
      redis_url_set: false,
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

    const secureJsonData: Record<string, string> = {};

    if (!state.postgresDsnSet) {
      secureJsonData.postgresDsn = state.postgresDsn;
    }

    if (!state.redis_url_set) {
      secureJsonData.redis_url = state.redis_url;
    }

    updatePluginAndReload(plugin.meta.id, {
      enabled,
      pinned,
      jsonData: {
        ...pluginJsonData,
        mimir_url: state.mimir_url,
        loki_url: state.loki_url,
        tempo_url: state.tempo_url,
      },
      secureJsonData: Object.keys(secureJsonData).length ? secureJsonData : undefined,
    });
  };

  return (
    <form onSubmit={onSubmit}>
      <FieldSet label="Forge Settings">
        <Field
          label="Postgres DSN"
          description="Connection string used by the backend to connect to Postgres"
          className={s.marginTop}
        >
          <SecretInput
            width={60}
            name="postgresDsn"
            id="config-postgres-dsn"
            value={state.postgresDsn}
            isConfigured={state.postgresDsnSet}
            placeholder={`E.g.: host=localhost user=forge password=secret dbname=forge sslmode=disable`}
            onChange={onChange}
            onReset={onResetPostgresDsn}
          />
        </Field>

        <Field
          label="Redis URL"
          description="Connection string used by the backend to connect to Redis"
          className={s.marginTop}
        >
          <SecretInput
            width={60}
            name="redis_url"
            id="config-redis-url"
            value={state.redis_url}
            isConfigured={state.redis_url_set}
            placeholder={`E.g.: redis://:password@localhost:6379/0`}
            onChange={onChange}
            onReset={onResetRedisUrl}
          />
        </Field>

        <Field
          label="Mimir URL"
          description="Prometheus remote_write endpoint URL included in generated Alloy config"
          className={s.marginTop}
        >
          <Input
            width={60}
            name="mimir_url"
            id="config-mimir-url"
            value={state.mimir_url}
            placeholder="E.g.: http://localhost:9009/api/v1/push"
            onChange={onChange}
          />
        </Field>

        <Field
          label="Loki URL"
          description="Loki write endpoint URL included in generated Alloy config"
          className={s.marginTop}
        >
          <Input
            width={60}
            name="loki_url"
            id="config-loki-url"
            value={state.loki_url}
            placeholder="E.g.: http://localhost:3100/loki/api/v1/push"
            onChange={onChange}
          />
        </Field>

        <Field
          label="Tempo URL"
          description="Tempo endpoint URL included in generated Alloy config"
          className={s.marginTop}
        >
          <Input
            width={60}
            name="tempo_url"
            id="config-tempo-url"
            value={state.tempo_url}
            placeholder="E.g.: http://localhost:4318"
            onChange={onChange}
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
