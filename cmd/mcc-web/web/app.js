const state = document.getElementById('state');
const stateDot = document.getElementById('state-dot');
const mccState = document.getElementById('mcc-state');
const connectedAt = document.getElementById('connected-at');
const attempts = document.getElementById('attempts');
const bridgeState = document.getElementById('bridge-state');
const log = document.getElementById('log');
const form = document.getElementById('command-form');
const input = document.getElementById('command');
const send = document.getElementById('send');
const clear = document.getElementById('clear');
let sequence = 0;
let socket;

function append(kind, text, detail = '') {
  const line = document.createElement('div');
  line.className = `line ${kind}`;
  const time = document.createElement('time');
  time.textContent = new Date().toLocaleTimeString();
  const body = document.createElement('span');
  body.textContent = text;
  line.append(time, body);
  if (detail) {
    const extra = document.createElement('small');
    extra.textContent = detail;
    line.append(extra);
  }
  log.appendChild(line);
  while (log.children.length > 500) log.firstChild.remove();
  log.scrollTop = log.scrollHeight;
}

function humanState(value) {
  return value ? value.charAt(0).toUpperCase() + value.slice(1) : 'Unknown';
}

function setStatus(status) {
  const value = status?.state || 'disconnected';
  state.textContent = value;
  mccState.textContent = humanState(value);
  stateDot.className = `dot ${value}`;
  attempts.textContent = status?.attempts ?? 0;
  connectedAt.textContent = status?.connectedAt ? new Date(status.connectedAt).toLocaleString() : '—';
  const ready = value === 'connected' && socket?.readyState === WebSocket.OPEN;
  input.disabled = !ready;
  send.disabled = !ready;
  if (status?.lastError) append('error', 'MCC connection error', status.lastError);
}

function summarizeEvent(message) {
  const data = message.data;
  if (typeof data === 'string') return data;
  if (data && typeof data === 'object') {
    const text = data.message ?? data.text ?? data.rawText ?? data.username ?? data.playerName;
    if (typeof text === 'string') return text;
    return JSON.stringify(data);
  }
  return data == null ? '' : String(data);
}

function connect() {
  const scheme = location.protocol === 'https:' ? 'wss' : 'ws';
  socket = new WebSocket(`${scheme}://${location.host}/ws`);
  bridgeState.textContent = 'Connecting to WebAdmin…';

  socket.addEventListener('open', () => {
    bridgeState.textContent = 'WebAdmin bridge connected';
    append('system', 'WebAdmin bridge connected');
  });

  socket.addEventListener('close', () => {
    bridgeState.textContent = 'WebAdmin bridge disconnected — retrying…';
    input.disabled = true;
    send.disabled = true;
    stateDot.className = 'dot disconnected';
    append('system', 'WebAdmin bridge disconnected');
    setTimeout(connect, 1500);
  });

  socket.addEventListener('error', () => append('error', 'WebAdmin WebSocket error'));

  socket.addEventListener('message', (event) => {
    let message;
    try { message = JSON.parse(event.data); } catch { append('event', event.data); return; }
    if (message.type === 'status') {
      setStatus(message.status);
      return;
    }
    if (message.type === 'command-response') {
      append(message.success ? 'response' : 'error', message.success ? `Command ${message.id} succeeded` : `Command ${message.id} failed`, message.message || '');
      return;
    }
    if (message.type === 'event') {
      append('event', message.event, summarizeEvent(message));
      return;
    }
    if (message.type === 'error') {
      append('error', 'Protocol error', message.message || 'Unknown error');
      return;
    }
    append('event', 'Unknown message', JSON.stringify(message));
  });
}

form.addEventListener('submit', (event) => {
  event.preventDefault();
  const value = input.value.trim();
  if (!value || socket?.readyState !== WebSocket.OPEN || input.disabled) return;
  const id = `ui-${++sequence}`;
  const payload = { type: 'command', id, command: value.startsWith('/') ? value : 'send', parameters: value.startsWith('/') ? [] : [value] };
  socket.send(JSON.stringify(payload));
  append('outgoing', value, id);
  input.value = '';
  input.focus();
});

clear.addEventListener('click', () => { log.textContent = ''; });
input.disabled = true;
send.disabled = true;
connect();
