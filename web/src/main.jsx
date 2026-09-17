import { render } from 'preact';
import './styles.css';
import App from './app.jsx';
import { loadSession } from './store.js';

loadSession().finally(() => {
  render(<App />, document.getElementById('app'));
});
