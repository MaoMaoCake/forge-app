import React, {useEffect, useState} from 'react';
import { css } from '@emotion/css';
import { GrafanaTheme2 } from '@grafana/data';
import { LinkButton, useStyles2 } from '@grafana/ui';
import { prefixRoute } from '../utils/utils.routing';
import { ROUTES } from '../constants';
import { testIds } from '../components/testIds';
import { PluginPage } from '@grafana/runtime';


function PageOne() {
  const s = useStyles2(getStyles);
  const [pingResult, setPingResult] = useState<string>('Loading...');

  useEffect(() => {
    const fetchPing = async () => {
      try {
        const res = await fetch('/api/plugins/maomaocake-forge-app/resources/ping', {
          method: 'GET',
        });

        if (!res.ok) {
          throw new Error(`Ping failed with status ${res.status}`);
        }

        // If backend returns plain text:
        // const text = await res.text();
        // setPingResult(text);

        // If backend returns JSON like { message: "pong" }:
        const data = await res.json();
        setPingResult(data.message ?? 'OK');
      } catch (err) {
        setPingResult('Ping request failed');
        // Optionally log `err`
      }
    };

    fetchPing();
  }, []);
  return (
    <PluginPage>
      <div data-testid={testIds.pageOne.container}>
        This is page one.
        <div className={s.marginTop}>
          Backend ping: {pingResult}
          <LinkButton data-testid={testIds.pageOne.navigateToFour} href={prefixRoute(ROUTES.Four)}>
            Full-width page example
          </LinkButton>
        </div>
      </div>
    </PluginPage>
  );
}

export default PageOne;

const getStyles = (theme: GrafanaTheme2) => ({
  marginTop: css`
    margin-top: ${theme.spacing(2)};
  `,
});
