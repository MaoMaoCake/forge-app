import React, { useState } from 'react';
import { useParams } from 'react-router-dom';
import { PluginPage } from '@grafana/runtime';
import { Input } from '@grafana/ui';

function PageConfig() {
  const { id } = useParams<{ id: string }>();
  const [inputValue, setInputValue] = useState(id || '');

  return (
    <PluginPage>
      <div style={{ maxWidth: 400, margin: '0 auto', padding: 24 }}>
        <h2>Config Page</h2>
        <div style={{ marginBottom: 16 }}>
          <strong>Current ID:</strong> {id ? id : <em>No ID in URL</em>}
        </div>
        <div>
          <label htmlFor="id-input" style={{ display: 'block', marginBottom: 8 }}>
            Enter new ID:
          </label>
          <Input
            id="id-input"
            value={inputValue}
            onChange={e => setInputValue(e.currentTarget.value)}
            placeholder="Type ID here"
            width={40}
          />
        </div>
      </div>
    </PluginPage>
  );
}

export default PageConfig;

