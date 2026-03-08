import React from 'react';
import { css } from '@emotion/css';
import { GrafanaTheme2 } from '@grafana/data';
import { ClipboardButton, Input, useStyles2 } from '@grafana/ui';
import { PluginPage } from '@grafana/runtime';
import { useParams } from 'react-router-dom';

function PageInstall() {
  const s = useStyles2(getStyles);
  const { uuid } = useParams<{ uuid: string }>();
  const placeholderUrl = `https://example.com/forge/install/${uuid ?? 'collector-id'}`;

  return (
    <PluginPage>
      <div className={s.container}>
        <h2>Install Collector</h2>
        <p className={s.bodyText}>
          Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et
          dolore magna aliqua.
        </p>
        <div className={s.urlRow}>
          <Input value={placeholderUrl} readOnly />
          <ClipboardButton icon="copy" variant="secondary" getText={() => placeholderUrl}>
            Copy
          </ClipboardButton>
        </div>
      </div>
    </PluginPage>
  );
}

export default PageInstall;

const getStyles = (theme: GrafanaTheme2) => ({
  container: css`
    padding: ${theme.spacing(2)};
    max-width: 720px;
  `,
  bodyText: css`
    color: ${theme.colors.text.secondary};
    margin-bottom: ${theme.spacing(2)};
  `,
  urlRow: css`
    display: grid;
    grid-template-columns: 1fr auto;
    gap: ${theme.spacing(1)};
    align-items: center;
  `,
});
