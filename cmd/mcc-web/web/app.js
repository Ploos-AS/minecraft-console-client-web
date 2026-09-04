const state = document.getElementById('state');
const log = document.getElementById('log');
const form = document.getElementById('command-form');
const input = document.getElementById('command');

function append(kind, payload) {
  const line = document.createElement('div');
  line.className = `line ${kind}`;
  const time = new Date().toLocaleTimeString();
  line.textContent = `[${time}] ${payload}`;
  log.appendChild(line);
  log.scrollTop = log.scrollHeight;
}

function setState(value) {
  state.textContent = value;
  state.className = `badge ${value}`;
}

const scheme = location.protocol === 'https:' ? 'wss' : 'ws';
const socket = new WebSocket(`${scheme}://${location.host}/ws`);

socket.addEventListener('open', () => {
  setState('bridge-online');
  append('system', 'Connected to MCC Web bridge');
});

socket.addEventListener('close', () => {
  setState('offline');
  append('system', 'Bridge connection closed');
});

socket.addEventListener('error', () => append('error', 'WebSocket error'));

socket.addEventListener('message', (event) => {
  try {
    const parsed = JSON.parse(event.data);
    if (parsed.type === 'mcc-web-status' && parsed.status) {
      setState(parsed.status.state);
      const suffix = parsed.status.lastError ? `: ${parsed.status.lastError}` : '';
      append('system', `MCC ${parsed.status.state}${suffix}`);
      return;
    }
    if (parsed.type === 'error') {
      append('error', parsed.message || event.data);
      return;
    }
    append('event', JSON.stringify(parsed));
  } catch {
    append('event', event.data);
  }
});

form.addEventListener('submit', (event) => {
  event.preventDefault();
  const value = input.value.trim();
  if (!value || socket.readyState !== WebSocket.OPEN) return;
  socket.send(value);
  append('outgoing', value);
  input.value = '';
});
