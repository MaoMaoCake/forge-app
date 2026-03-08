import React, { useEffect, useState } from 'react';
import { PluginPage } from '@grafana/runtime';
import { css } from '@emotion/css';
import { GrafanaTheme2 } from '@grafana/data';
import { Alert, Button, Field, Input, Spinner, Switch, TextArea, useStyles2 } from '@grafana/ui';
import { testIds } from '../components/testIds';
import { useParams } from 'react-router-dom';

// Types mirroring the Go structs for Agent and AgentConfig (simplified)
interface AgentConfig {
  config?: string;
  collectUnixLogs?: boolean;
  collectUnixNodeMetrics?: boolean;
  collectWinLogs?: boolean;
  collectWinNodeMetrics?: boolean;
  collectCadvisorMetrics?: boolean;
  collectKubernetesMetrics?: boolean;
}

interface AgentPayload {
  agent_uuid: string;
  name?: string;
  advanced?: boolean;
  agent_config: AgentConfig;
}

interface LegacyAgentConfig {
  Config?: string;
  CollectUnixLogs?: boolean;
  CollectUnixNodeMetrics?: boolean;
  CollectWinLogs?: boolean;
  CollectWinNodeMetrics?: boolean;
  CollectCadvisorMetrics?: boolean;
  CollectKubernetesMetrics?: boolean;
}

interface CollectorResponse {
  agent_uuid?: string;
  uuid?: string;
  name?: string;
  advanced?: boolean;
  AgentConfig?: LegacyAgentConfig;
  agent_config?: AgentConfig;
  agentConfig?: AgentConfig;
}

function PageConfig() {
  const s = useStyles2(getStyles);
  const { uuid } = useParams<{ uuid: string }>();

  const [form, setForm] = useState<AgentPayload>({
    agent_uuid: uuid || '',
    name: '',
    advanced: false,
    agent_config: {
      config: '',
      collectUnixLogs: false,
      collectUnixNodeMetrics: false,
      collectWinLogs: false,
      collectWinNodeMetrics: false,
      collectCadvisorMetrics: false,
      collectKubernetesMetrics: false,
    },
  });

  const [submitting, setSubmitting] = useState(false);
  const [loadingExisting, setLoadingExisting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);

  // Load existing agent details when UUID is provided via route param
  useEffect(() => {
    if (!uuid) {
      return;
    }

    const loadAgent = async () => {
      setLoadingExisting(true);
      setError(null);
      try {
        const res = await fetch(
          `/api/plugins/maomaocake-forge-app/resources/collector?uuid=${encodeURIComponent(uuid)}`,
          {
            method: 'GET',
          }
        );

        if (res.status === 404) {
          setError('Collector not found');
          return;
        }

        if (!res.ok) {
          throw new Error(`Failed to load collector: ${res.status}`);
        }

        const agent: CollectorResponse = await res.json();
        const legacyConfig = agent.AgentConfig;
        const camelConfig = agent.agentConfig;
        const underscoreConfig = agent.agent_config;

        setForm({
          agent_uuid: agent.agent_uuid || agent.uuid || uuid,
          name: agent.name || '',
          advanced: !!agent.advanced,
          agent_config: {
            config:
              legacyConfig?.Config ??
              underscoreConfig?.config ??
              camelConfig?.config ??
              '',
            collectUnixLogs:
              legacyConfig?.CollectUnixLogs ??
              underscoreConfig?.collectUnixLogs ??
              camelConfig?.collectUnixLogs ??
              false,
            collectUnixNodeMetrics:
              legacyConfig?.CollectUnixNodeMetrics ??
              underscoreConfig?.collectUnixNodeMetrics ??
              camelConfig?.collectUnixNodeMetrics ??
              false,
            collectWinLogs:
              legacyConfig?.CollectWinLogs ??
              underscoreConfig?.collectWinLogs ??
              camelConfig?.collectWinLogs ??
              false,
            collectWinNodeMetrics:
              legacyConfig?.CollectWinNodeMetrics ??
              underscoreConfig?.collectWinNodeMetrics ??
              camelConfig?.collectWinNodeMetrics ??
              false,
            collectCadvisorMetrics:
              legacyConfig?.CollectCadvisorMetrics ??
              underscoreConfig?.collectCadvisorMetrics ??
              camelConfig?.collectCadvisorMetrics ??
              false,
            collectKubernetesMetrics:
              legacyConfig?.CollectKubernetesMetrics ??
              underscoreConfig?.collectKubernetesMetrics ??
              camelConfig?.collectKubernetesMetrics ??
              false,
          },
        });
      } catch (err: any) {
        setError(err?.message ?? 'Failed to load existing collector');
      } finally {
        setLoadingExisting(false);
      }
    };

    loadAgent();
  }, [uuid]);

  const updateAgentField = (key: Exclude<keyof AgentPayload, 'agent_uuid'>, value: any) => {
    setForm((prev) => ({
      ...prev,
      [key]: value,
    }));
  };

  const updateConfigField = (key: keyof AgentConfig, value: any) => {
    setForm((prev) => ({
      ...prev,
      agent_config: {
        ...prev.agent_config,
        [key]: value,
      },
    }));
  };

  const handleSubmit = async (evt: React.FormEvent) => {
    evt.preventDefault();
    setError(null);
    setSuccess(null);

    if (!form.agent_uuid) {
      setError('Agent UUID is required');
      return;
    }

    setSubmitting(true);
    try {
      const res = await fetch('/api/plugins/maomaocake-forge-app/resources/collector', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          agent_uuid: form.agent_uuid,
          name: form.name,
          advanced: form.advanced,
          agent_config: form.agent_config,
        }),
      });

      if (!res.ok) {
        const text = await res.text();
        throw new Error(text || `Request failed with status ${res.status}`);
      }

      setSuccess('Collector configuration saved successfully');
    } catch (err: any) {
      setError(err?.message ?? 'Failed to save collector configuration');
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <PluginPage>
      <div className={s.container} data-testid={testIds.appConfig.container}>
        <h2>Edit Collector</h2>
        <p>Update settings for an existing collector. UUID cannot be changed.</p>

        {(loadingExisting || submitting) && !error && (
          <div className={s.alertWrapper}>
            <Spinner size={16} />
          </div>
        )}

        {error && (
          <div className={s.alertWrapper}>
            <Alert title="Error" severity="error">
              {error}
            </Alert>
          </div>
        )}

        {success && (
          <div className={s.alertWrapper}>
            <Alert title="Success" severity="success">
              {success}
            </Alert>
          </div>
        )}

        <form className={s.form} onSubmit={handleSubmit}>
          <Field
            label="Agent UUID"
            required
            description="Identifier for this collector (read-only)."
          >
            <Input value={form.agent_uuid} disabled />
          </Field>

          <Field label="Name" description="Human-readable name for the collector.">
            <Input
              value={form.name}
              onChange={(e) => updateAgentField('name', e.currentTarget.value)}
            />
          </Field>

          <Field label="Advanced" description="Mark this collector as advanced.">
            <Switch
              value={!!form.advanced}
              onChange={(e) => updateAgentField('advanced', e.currentTarget.checked)}
            />
          </Field>

          <h3>Collection Settings</h3>

          <Field label="Unix Logs">
            <Switch
              value={!!form.agent_config.collectUnixLogs}
              onChange={(e) => updateConfigField('collectUnixLogs', e.currentTarget.checked)}
            />
          </Field>

          <Field label="Unix Node Metrics">
            <Switch
              value={!!form.agent_config.collectUnixNodeMetrics}
              onChange={(e) => updateConfigField('collectUnixNodeMetrics', e.currentTarget.checked)}
            />
          </Field>

          <Field label="Windows Logs">
            <Switch
              value={!!form.agent_config.collectWinLogs}
              onChange={(e) => updateConfigField('collectWinLogs', e.currentTarget.checked)}
            />
          </Field>

          <Field label="Windows Node Metrics">
            <Switch
              value={!!form.agent_config.collectWinNodeMetrics}
              onChange={(e) => updateConfigField('collectWinNodeMetrics', e.currentTarget.checked)}
            />
          </Field>

          <Field label="cAdvisor Metrics">
            <Switch
              value={!!form.agent_config.collectCadvisorMetrics}
              onChange={(e) => updateConfigField('collectCadvisorMetrics', e.currentTarget.checked)}
            />
          </Field>

          <Field label="Kubernetes Metrics">
            <Switch
              value={!!form.agent_config.collectKubernetesMetrics}
              onChange={(e) => updateConfigField('collectKubernetesMetrics', e.currentTarget.checked)}
            />
          </Field>

          <Field
            label="Raw Config"
            description="Alloy config for this collector. This is only used if advanced is enabled."
          >
            <TextArea
              rows={6}
              value={form.agent_config.config || ''}
              onChange={(e) => updateConfigField('config', e.currentTarget.value)}
            />
          </Field>

          <div className={s.actions}>
            <Button type="submit" disabled={submitting} variant="primary">
              {submitting ? 'Saving...' : 'Save Collector'}
            </Button>
          </div>
        </form>
      </div>
    </PluginPage>
  );
}

export default PageConfig;

const getStyles = (theme: GrafanaTheme2) => ({
  container: css`
    padding: ${theme.spacing(2)};
    max-width: 800px;
  `,
  form: css`
    display: flex;
    flex-direction: column;
    gap: ${theme.spacing(2)};
  `,
  actions: css`
    margin-top: ${theme.spacing(2)};
  `,
  alertWrapper: css`
    margin-bottom: ${theme.spacing(2)};
  `,
});

