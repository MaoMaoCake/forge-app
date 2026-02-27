import React, { useEffect, useState } from 'react';
import { css } from '@emotion/css';
import { GrafanaTheme2 } from '@grafana/data';
import { Alert, Spinner, useStyles2 } from '@grafana/ui';
import { PluginPage } from '@grafana/runtime';
import { testIds } from '../components/testIds';

// Shape of a single Agent returned by the /collector endpoint.
// This mirrors the Go structs (Agent with embedded AgentConfig) loosely.
interface AgentConfig {
  id?: number;
  uuid?: string;
  name?: string;
  description?: string;
  // Add more fields if you expose them from the backend.
}

interface Agent {
  id?: number;
  uuid?: string;
  name?: string;
  description?: string;
  agentConfig?: AgentConfig | AgentConfig[];
  AgentConfig?: AgentConfig | AgentConfig[]; // handle different casings just in case
}

function PageOverview() {
  const s = useStyles2(getStyles);
  const [agents, setAgents] = useState<Agent[]>([]);
  const [loading, setLoading] = useState<boolean>(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const fetchCollectors = async () => {
      try {
        const res = await fetch('/api/plugins/maomaocake-forge-app/resources/collector', {
          method: 'GET',
        });

        if (!res.ok) {
          throw new Error(`Request failed with status ${res.status}`);
        }

        const data = await res.json();
        // Expecting an array of agents
        setAgents(Array.isArray(data) ? data : []);
      } catch (err: any) {
        setError(err?.message || 'Failed to load collectors');
      } finally {
        setLoading(false);
      }
    };

    fetchCollectors();
  }, []);

  const renderAgentName = (agent: Agent, index: number) => {
    // Prefer explicit name/uuid, but always show something.
    const base = agent.name || agent.uuid || `Collector #${index + 1}`;
    const description = agent.description;

    return (
      <div>
        <div>{base}</div>
        {description && <div className={s.description}>{description}</div>}
      </div>
    );
  };

  return (
    <PluginPage>
      <div className={s.container} data-testid={testIds.pageOverview.container}>
        <h2>Collectors</h2>

        {loading && (
          <div className={s.center} data-testid={testIds.pageOverview.loading}>
            <Spinner /> Loading collectors...
          </div>
        )}

        {error && !loading && (
          <div data-testid={testIds.pageOverview.error}>
            <Alert title="Failed to load collectors" severity="error">
              {error}
            </Alert>
          </div>
        )}

        {!loading && !error && (
          <>
            {agents.length === 0 ? (
              <div>No collectors found.</div>
            ) : (
              <ul className={s.list} data-testid={testIds.pageOverview.list}>
                {agents.map((agent, idx) => (
                  <li
                    key={agent.uuid || agent.id || idx}
                    className={s.listItem}
                    data-testid={testIds.pageOverview.listItem}
                  >
                    {renderAgentName(agent, idx)}
                  </li>
                ))}
              </ul>
            )}
          </>
        )}
      </div>
    </PluginPage>
  );
}

export default PageOverview;

const getStyles = (theme: GrafanaTheme2) => ({
  container: css`
    padding: ${theme.spacing(2)};
  `,
  list: css`
    margin-top: ${theme.spacing(2)};
    list-style: none;
    padding-left: 0;
  `,
  listItem: css`
    padding: ${theme.spacing(1)} 0;
    border-bottom: 1px solid ${theme.colors.border.weak};
  `,
  description: css`
    color: ${theme.colors.text.secondary};
    font-size: ${theme.typography.bodySmall.fontSize};
  `,
  center: css`
    display: flex;
    align-items: center;
    gap: ${theme.spacing(1)};
  `,
});

