import React, { useEffect, useState } from 'react';
import { css } from '@emotion/css';
import { GrafanaTheme2 } from '@grafana/data';
import { Alert, LinkButton, Spinner, useStyles2 } from '@grafana/ui';
import { PluginPage } from '@grafana/runtime';
import { testIds } from '../components/testIds';
import { prefixRoute } from '../utils/utils.routing';
import { ROUTES } from '../constants';

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
  uuid?: string; // optional, for future compatibility
  agent_uuid?: string; // actual identifier from backend JSON
  name?: string;
  description?: string;
  last_seen?: string;
  lastSeen?: string;
  LastSeen?: string;
  last_seen_version?: string;
  lastSeenVersion?: string;
  LastSeenVersion?: string;
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

  const formatLastSeen = (value?: string) => {
    if (!value) {
      return null;
    }

    const parsed = new Date(value);
    if (Number.isNaN(parsed.getTime())) {
      return value;
    }

    return parsed.toLocaleString();
  };

  const renderAgentName = (agent: Agent, index: number) => {
    // Prefer explicit name/UUID, but always show something.
    const base = agent.name || agent.agent_uuid || agent.uuid || `Collector #${index + 1}`;
    const description = agent.description;
    const lastSeen = formatLastSeen(agent.last_seen || agent.lastSeen || agent.LastSeen);
    const version = agent.last_seen_version || agent.lastSeenVersion || agent.LastSeenVersion;

    return (
      <div>
        <div>{base}</div>
        {description && <div className={s.description}>{description}</div>}
        {lastSeen && <div className={s.meta}>Last seen: {lastSeen}</div>}
        {version && <div className={s.meta}>Last seen Alloy version: {version}</div>}
      </div>
    );
  };

  const getEditHref = (agent: Agent) => {
    const id = agent.agent_uuid || agent.uuid;
    if (!id) {
      return undefined;
    }
    return prefixRoute(`${ROUTES.config}/${id}`);
  };

  const getInstallHref = (agent: Agent) => {
    const id = agent.agent_uuid || agent.uuid;
    if (!id) {
      return undefined;
    }
    return prefixRoute(`${ROUTES.install}/${id}`);
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
                {agents.map((agent, idx) => {
                  const editHref = getEditHref(agent);
                  const installHref = getInstallHref(agent);
                  return (
                    <li
                      key={agent.agent_uuid || agent.uuid || agent.id || idx}
                      className={s.listItem}
                      data-testid={testIds.pageOverview.listItem}
                    >
                      <div className={s.listItemInner}>
                        {renderAgentName(agent, idx)}
                        {(editHref || installHref) && (
                          <div className={s.actionGroup}>
                            {editHref && (
                              <LinkButton variant="secondary" size="sm" href={editHref}>
                                Edit
                              </LinkButton>
                            )}
                            {installHref && (
                              <LinkButton variant="secondary" size="sm" href={installHref}>
                                Install
                              </LinkButton>
                            )}
                          </div>
                        )}
                      </div>
                    </li>
                  );
                })}
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
  listItemInner: css`
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: ${theme.spacing(2)};
  `,
  actionGroup: css`
    display: flex;
    align-items: center;
    gap: ${theme.spacing(1)};
  `,
  description: css`
    color: ${theme.colors.text.secondary};
    font-size: ${theme.typography.bodySmall.fontSize};
  `,
  meta: css`
    color: ${theme.colors.text.secondary};
    font-size: ${theme.typography.bodySmall.fontSize};
  `,
  center: css`
    display: flex;
    align-items: center;
    gap: ${theme.spacing(1)};
  `,
});
