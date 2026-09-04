const state = document.getElementById('state');
const stateDot = document.getElementById('state-dot');
const mccState = document.getElementById('mcc-state');
const connectedAt = document.getElementById('connected-at');
const attempts = document.getElementById('attempts');
const health = document.getElementById('health');
const food = document.getElementById('food');
const level = document.getElementById('level');
const xp = document.getElementById('xp');
const tps = document.getElementById('tps');
const worldTime = document.getElementById('world-time');
const players = document.getElementById('players');
const lastDisconnect = document.getElementById('last-disconnect');
const bridgeState = document.getElementById('bridge-state');
const log = document.getElementById('log');
const chat = document.getElementById('chat');
const commandForm = document.getElementById('command-form');
const commandInput = document.getElementById('command');
const commandSend = document.getElementById('send');
const chatForm = document.getElementById('chat-form');
const chatInput = document.getElementById('chat-input');
const chatSend = document.getElementById('chat-send');
const clear = document.getElementById('clear');
const clearChat = document.getElementById('clear-chat');
const logout = document.getElementById('logout');
const observedPlayers = new Map();
let sequence = 0;
let socket;

function boundedAppend(container, node, limit) {
  container.appendChild(node);
  while (container.children.length > limit) container.firstChild.remove();
  container.scrollTop = container.scrollHeight;
}

function appendActivity(kind, text, detail = '') {
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
  boundedAppend(log, line, 500);
}

function appendChat(kind, sender, message) {
  const row = document.createElement('div');
  row.className = `chat-row ${kind}`;
  const meta = document.createElement('div');
  const time = document.createElement('time');
  time.textContent = new Date().toLocaleTimeString();
  meta.appendChild(time);
  if (sender) {
    const name = document.createElement('strong');
    name.textContent = sender;
    meta.appendChild(name);
  }
  const body = document.createElement('p');
  body.textContent = message;
  row.append(meta, body);
  boundedAppend(chat, row, 300);
}

function humanState(value) {
  return value ? value.charAt(0).toUpperCase() + value.slice(1) : 'Unknown';
}

function setControlsReady(ready) {
  commandInput.disabled = !ready;
  commandSend.disabled = !ready;
  chatInput.disabled = !ready;
  chatSend.disabled = !ready;
}

function setStatus(status) {
  const value = status?.state || 'disconnected';
  state.textContent = value;
  mccState.textContent = humanState(value);
  stateDot.className = `dot ${value}`;
  attempts.textContent = status?.attempts ?? 0;
  connectedAt.textContent = status?.connectedAt ? new Date(status.connectedAt).toLocaleString() : '—';
  const ready = value === 'connected' && socket?.readyState === WebSocket.OPEN;
  setControlsReady(ready);
  if (status?.lastError) appendActivity('error', 'MCC connection error', status.lastError);
}

function summarizeEvent(message) {
  const data = message.data;
  if (typeof data === 'string') return data;
  if (data && typeof data === 'object') {
    const text = data.message ?? data.text ?? data.rawText ?? data.username ?? data.playerName ?? data.name;
    if (typeof text === 'string') return text;
    return JSON.stringify(data);
  }
  return data == null ? '' : String(data);
}

function formatWorldTime(value) {
  if (!Number.isFinite(value)) return '—';
  const ticks = ((value % 24000) + 24000) % 24000;
  const totalMinutes = Math.floor(((ticks + 6000) % 24000) / 1000 * 60);
  const hours = Math.floor(totalMinutes / 60) % 24;
  const minutes = totalMinutes % 60;
  return `${String(hours).padStart(2, '0')}:${String(minutes).padStart(2, '0')}`;
}

function updateStructuredEvent(message) {
  const data = message.data && typeof message.data === 'object' ? message.data : {};
  switch (message.event) {
    case 'OnChatPublic':
      appendChat('public', data.sender || '', data.message || data.rawText || '');
      break;
    case 'OnChatPrivate':
      appendChat('private', data.sender || '', data.message || data.rawText || '');
      break;
    case 'OnChatRaw':
      if (data.text) appendChat('raw', '', data.text);
      break;
    case 'OnHealthUpdate':
      health.textContent = Number.isFinite(data.health) ? Number(data.health).toFixed(1) : '—';
      food.textContent = Number.isFinite(data.food) ? data.food : '—';
      break;
    case 'OnSetExperience':
      level.textContent = Number.isFinite(data.level) ? data.level : '—';
      xp.textContent = Number.isFinite(data.totalExperience) ? data.totalExperience : '—';
      break;
    case 'OnServerTpsUpdate':
      tps.textContent = Number.isFinite(data.tps) ? Number(data.tps).toFixed(1) : '—';
      break;
    case 'OnTimeUpdate':
      worldTime.textContent = formatWorldTime(data.timeOfDay);
      break;
    case 'OnPlayerJoin':
      if (data.uuid || data.name) observedPlayers.set(data.uuid || data.name, data.name || data.uuid);
      players.textContent = observedPlayers.size;
      appendChat('system', '', `${data.name || 'Player'} joined`);
      break;
    case 'OnPlayerLeave':
      if (data.uuid) observedPlayers.delete(data.uuid);
      else if (data.name) {
        for (const [id, name] of observedPlayers) if (name === data.name) observedPlayers.delete(id);
      }
      players.textContent = observedPlayers.size;
      appendChat('system', '', `${data.name || 'Player'} left`);
      break;
    case 'OnDisconnect':
      lastDisconnect.textContent = data.message || data.reason || 'Disconnected';
      break;
    case 'OnGameJoined':
      lastDisconnect.textContent = '—';
      break;
  }
}

function sendCommand(command, parameters, display) {
  if (socket?.readyState !== WebSocket.OPEN) return false;
  const id = `ui-${++sequence}`;
  socket.send(JSON.stringify({ type: 'command', id, command, parameters }));
  appendActivity('outgoing', display, id);
  return true;
}

function connect() {
  const scheme = location.protocol === 'https:' ? 'wss' : 'ws';
  socket = new WebSocket(`${scheme}://${location.host}/ws`);
  bridgeState.textContent = 'Connecting to WebAdmin…';

  socket.addEventListener('open', () => {
    bridgeState.textContent = 'WebAdmin bridge connected';
    appendActivity('system', 'WebAdmin bridge connected');
  });

  socket.addEventListener('close', (event) => {
    if (event.code === 1008 || event.code === 4401) {
      location.assign('/login');
      return;
    }
    bridgeState.textContent = 'WebAdmin bridge disconnected — retrying…';
    setControlsReady(false);
    stateDot.className = 'dot disconnected';
    appendActivity('system', 'WebAdmin bridge disconnected');
    setTimeout(connect, 1500);
  });

  socket.addEventListener('error', () => appendActivity('error', 'WebAdmin WebSocket error'));

  socket.addEventListener('message', (event) => {
    let message;
    try { message = JSON.parse(event.data); } catch { appendActivity('event', event.data); return; }
    if (message.type === 'status') {
      setStatus(message.status);
      return;
    }
    if (message.type === 'command-response') {
      appendActivity(message.success ? 'response' : 'error', message.success ? `Command ${message.id} succeeded` : `Command ${message.id} failed`, message.message || '');
      return;
    }
    if (message.type === 'event') {
      updateStructuredEvent(message);
      appendActivity('event', message.event, summarizeEvent(message));
      return;
    }
    if (message.type === 'error') {
      appendActivity('error', 'Protocol error', message.message || 'Unknown error');
      return;
    }
    appendActivity('event', 'Unknown message', JSON.stringify(message));
  });
}

chatForm.addEventListener('submit', (event) => {
  event.preventDefault();
  const value = chatInput.value.trim();
  if (!value || chatInput.disabled) return;
  if (sendCommand('send', [value], value)) {
    appendChat('outgoing', 'You', value);
    chatInput.value = '';
    chatInput.focus();
  }
});

commandForm.addEventListener('submit', (event) => {
  event.preventDefault();
  const value = commandInput.value.trim();
  if (!value || commandInput.disabled) return;
  const command = value.startsWith('/') ? value : `/${value}`;
  if (sendCommand(command, [], command)) {
    commandInput.value = '';
    commandInput.focus();
  }
});

clear.addEventListener('click', () => { log.textContent = ''; });
clearChat.addEventListener('click', () => { chat.textContent = ''; });
logout.addEventListener('click', async () => {
  logout.disabled = true;
  try {
    await fetch('/api/logout', { method: 'POST', credentials: 'same-origin' });
  } finally {
    location.assign('/login');
  }
});

setControlsReady(false);
connect();
