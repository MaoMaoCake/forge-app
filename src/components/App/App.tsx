import React from 'react';
import { Route, Routes } from 'react-router-dom';
import { AppRootProps } from '@grafana/data';
import { ROUTES } from '../../constants';
const PageOne = React.lazy(() => import('../../pages/PageOne'));
const PageTwo = React.lazy(() => import('../../pages/PageTwo'));
const PageThree = React.lazy(() => import('../../pages/PageThree'));
const PageFour = React.lazy(() => import('../../pages/PageFour'));
const PageConfig = React.lazy(() => import('../../pages/PageConfig'));
const PageCreate = React.lazy(() => import('../../pages/PageCreate'));
const PageOverview = React.lazy(() => import('../../pages/PageOverview'));
const PageInstall = React.lazy(() => import('../../pages/PageInstall'));

function App(props: AppRootProps) {
  return (
    <Routes>
      <Route path={ROUTES.Two} element={<PageTwo />} />
      <Route path={`${ROUTES.Three}/:id?`} element={<PageThree />} />

      {/* Full-width page (this page will have no side navigation) */}
      <Route path={ROUTES.Four} element={<PageFour />} />

      {/* Default page */}
      <Route path="*" element={<PageOne />} />
      <Route path={ROUTES.dashboard} element={<PageOverview />} />
      {/* Edit page with UUID param, reached from overview */}
      <Route path={`${ROUTES.config}/:uuid`} element={<PageConfig />} />
      <Route path={`${ROUTES.install}/:uuid`} element={<PageInstall />} />
      <Route path={ROUTES.create} element={<PageCreate />} />
    </Routes>
  );
}

export default App;
