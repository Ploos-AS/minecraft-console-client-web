const state = document.getElementById('state');
const stateDot = document.getElementById('state-dot');
const mccState = document.getElementById('mcc-state');
const connectedAt = document.getElementById('connected-at');
const attempts = document.getElementById('attempts');
const username = document.getElementById('username');
const server = document.getElementById('server');
const gamemode = document.getElementById('gamemode');
const protocol = document.getElementById('protocol');
const locationValue = document.getElementById('location');
const health = document.getElementById('health');
const food = document.getElementById('food');
const level = document.getElementById('level');
const xp = document.getElementById('xp');
const tps = document.getElementById('tps');
const worldTime = document.getElementById('world-time');
const players = document.getElementById('players');
const playerList = document.getElementById('player-list');
const lastDisconnect = document.getElementById('last-disconnect');
const hydrationState = document.getElementById('hydration-state');
const uuid = document.getElementById('uuid');
const currentSlot = document.getElementById('current-slot');
const inventoryEnabled = document.getElementById('inventory-enabled');
const selfLatency = document.getElementById('self-latency');
const inventory = document.getElementById('inventory');
const inventorySummary = document.getElementById('inventory-summary');
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
const hydrationCommands = [
  ['username', 'GetUsername'],
  ['uuid', 'GetUserUUID'],
  ['server-host', 'GetServerHost'],
  ['server-port', 'GetServerPort'],
  ['gamemode', 'GetGamemode'],
  ['location', 'GetCurrentLocation'],
  ['players', 'GetOnlinePlayers'],
  ['latency', 'GetPlayersLatency'],
  ['tps', 'GetServerTPS'],
  ['protocol', 'GetProtocolVersion'],
  ['inventory-enabled', 'GetInventoryEnabled'],
  ['inventory', 'GetPlayerInventory'],
  ['current-slot', 'GetCurrentSlot'],
];
let sequence = 0;
let socket;
let hydratedConnection = '';
let hydrationPending = new Set();
let serverHost = '';
let serverPort = '';

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

function resetInventory() {
  inventory.textContent = '';
  inventorySummary.textContent = 'Waiting';
  inventoryEnabled.textContent = '—';
  currentSlot.textContent = '—';
}

function resetAuthoritativeState() {
  username.textContent = '—';
  uuid.textContent = '—';
  server.textContent = '—';
  gamemode.textContent = '—';
  protocol.textContent = '—';
  locationValue.textContent = '—';
  players.textContent = '—';
  playerList.textContent = '—';
  selfLatency.textContent = '—';
  serverHost = '';
  serverPort = '';
  hydrationPending.clear();
  resetInventory();
}

function updateServerLabel() {
  if (!serverHost && !serverPort) {
    server.textContent = '—';
    return;
  }
  server.textContent = serverPort ? `${serverHost || '?'}:${serverPort}` : serverHost;
  server.title = server.textContent;
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
  if (!ready) {
    hydratedConnection = '';
    hydrationState.textContent = 'Waiting for MCC';
  }
  if (ready && status?.connectedAt && hydratedConnection !== status.connectedAt) {
    hydratedConnection = status.connectedAt;
    beginHydration();
  }
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

function formatGamemode(value) {
  const names = ['Survival', 'Creative', 'Adventure', 'Spectator'];
  return Number.isInteger(value) && names[value] ? names[value] : String(value ?? '—');
}

function formatLocation(data) {
  if (!data || typeof data !== 'object') return '—';
  const x = Number(data.x ?? data.X);
  const y = Number(data.y ?? data.Y);
  const z = Number(data.z ?? data.Z);
  if (![x, y, z].every(Number.isFinite)) return '—';
  return `${x.toFixed(1)}, ${y.toFixed(1)}, ${z.toFixed(1)}`;
}

function parseResponseMessage(message) {
  if (!message) return null;
  try { return JSON.parse(message); } catch { return message; }
}

function sendCommand(command, id = `ui-${++sequence}`, parameters = []) {
  if (socket?.readyState !== WebSocket.OPEN) return false;
  socket.send(JSON.stringify({ type: 'command', id, command, parameters }));
  return true;
}

function itemType(item) {
  const value = item?.type ?? item?.Type ?? item?.itemType ?? item?.ItemType ?? 'Unknown';
  return typeof value === 'object' ? JSON.stringify(value) : String(value);
}

function itemCount(item) {
  const value = Number(item?.count ?? item?.Count ?? 1);
  return Number.isFinite(value) ? value : 1;
}

function renderInventory(data) {
  inventory.textContent = '';
  const items = data?.items ?? data?.Items ?? data;
  if (!items || typeof items !== 'object' || Array.isArray(items)) {
    inventorySummary.textContent = 'Unavailable';
    return;
  }
  const entries = Object.entries(items)
    .map(([slot, item]) => [Number(slot), item])
    .filter(([slot, item]) => Number.isFinite(slot) && item)
    .sort((a, b) => a[0] - b[0]);
  inventorySummary.textContent = `${entries.length} occupied`;
  if (!entries.length) {
    const empty = document.createElement('p');
    empty.className = 'inventory-empty';
    empty.textContent = 'No occupied inventory slots reported';
    inventory.appendChild(empty);
    return;
  }
  for (const [slot, item] of entries) {
    const card = document.createElement('div');
    card.className = 'inventory-slot';
    if (Number(currentSlot.textContent) === slot) card.classList.add('selected');
    const slotLabel = document.createElement('span');
    slotLabel.textContent = `Slot ${slot}`;
    const name = document.createElement('strong');
    name.textContent = itemType(item);
    name.title = name.textContent;
    const count = document.createElement('small');
    count.textContent = `×${itemCount(item)}`;
    card.append(slotLabel, name, count);
    inventory.appendChild(card);
  }
}

function beginHydration() {
  resetAuthoritativeState();
  hydrationPending = new Set(hydrationCommands.map(([key]) => key));
  hydrationState.textContent = `Refreshing ${hydrationPending.size} fields…`;
  for (const [key, command] of hydrationCommands) {
    if (!sendCommand(command, `hydrate:${key}`)) hydrationPending.delete(key);
  }
  if (hydrationPending.size === 0) hydrationState.textContent = 'Refresh unavailable';
}

function finishHydrationField(key) {
  hydrationPending.delete(key);
  hydrationState.textContent = hydrationPending.size === 0 ? 'Authoritative state loaded' : `Refreshing ${hydrationPending.size} fields…`;
}

function handleHydrationResponse(message) {
  if (!message.id?.startsWith('hydrate:')) return false;
  const key = message.id.slice('hydrate:'.length);
  if (!message.success) {
    appendActivity('error', `State query ${key} failed`, message.message || '');
    if (key === 'inventory') inventorySummary.textContent = 'Unavailable';
    finishHydrationField(key);
    return true;
  }
  const data = parseResponseMessage(message.message);
  switch (key) {
    case 'username':
      username.textContent = data?.username || '—';
      break;
    case 'uuid':
      uuid.textContent = data?.uuid || String(data ?? '—');
      break;
    case 'server-host':
      serverHost = data?.host || '';
      updateServerLabel();
      break;
    case 'server-port':
      serverPort = data?.port ?? '';
      updateServerLabel();
      break;
    case 'gamemode':
      gamemode.textContent = formatGamemode(data?.gamemode);
      break;
    case 'location':
      locationValue.textContent = formatLocation(data);
      break;
    case 'players': {
      const list = Array.isArray(data) ? data : [];
      players.textContent = String(list.length);
      playerList.textContent = list.length ? list.join(', ') : 'No players reported';
      observedPlayers.clear();
      for (const name of list) observedPlayers.set(name, name);
      break;
    }
    case 'latency': {
      const values = data && typeof data === 'object' ? data : {};
      const own = values[username.textContent];
      selfLatency.textContent = Number.isFinite(own) ? `${own} ms` : '—';
      break;
    }
    case 'tps':
      tps.textContent = Number.isFinite(data?.tps) ? Number(data.tps).toFixed(1) : '—';
      break;
    case 'protocol':
      protocol.textContent = data?.protocolVersion ?? '—';
      break;
    case 'inventory-enabled':
      inventoryEnabled.textContent = data?.enabled === true ? 'Enabled' : data?.enabled === false ? 'Disabled' : '—';
      break;
    case 'inventory':
      renderInventory(data);
      break;
    case 'current-slot':
      currentSlot.textContent = data?.slot ?? data?.currentSlot ?? String(data ?? '—');
      break;
  }
  finishHydrationField(key);
  return true;
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
    case 'OnGamemodeUpdate':
      if (data.playerName === username.textContent) gamemode.textContent = formatGamemode(data.gamemode);
      break;
    case 'OnPlayerJoin':
      if (data.uuid || data.name) observedPlayers.set(data.uuid || data.name, data.name || data.uuid);
      players.textContent = String(observedPlayers.size);
      playerList.textContent = [...observedPlayers.values()].join(', ') || 'No players reported';
      appendChat('system', '', `${data.name || 'Player'} joined`);
      break;
    case 'OnPlayerLeave':
      if (data.uuid) observedPlayers.delete(data.uuid);
      else if (data.name) for (const [id, name] of observedPlayers) if (name === data.name) observedPlayers.delete(id);
      players.textContent = String(observedPlayers.size);
      playerList.textContent = [...observedPlayers.values()].join(', ') || 'No players reported';
      appendChat('system', '', `${data.name || 'Player'} left`);
      break;
    case 'OnDisconnect':
      lastDisconnect.textContent = data.message || data.reason || 'Disconnected';
      resetAuthoritativeState();
      hydrationState.textContent = 'Waiting for MCC';
      break;
    case 'OnGameJoined':
      lastDisconnect.textContent = '—';
      if (socket?.readyState === WebSocket.OPEN) beginHydration();
      break;
    case 'OnInventoryUpdate':
    case 'OnHeldItemChange':
      if (socket?.readyState === WebSocket.OPEN) {
        sendCommand('GetPlayerInventory', 'hydrate:inventory');
        sendCommand('GetCurrentSlot', 'hydrate:current-slot');
      }
      break;
  }
}

function sendText(text, display) {
  if (socket?.readyState !== WebSocket.OPEN) return false;
  const id = `ui-${++sequence}`;
  socket.send(JSON.stringify({ type: 'text', id, text }));
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
    hydratedConnection = '';
    resetAuthoritativeState();
    hydrationState.textContent = 'Waiting for MCC';
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
      if (handleHydrationResponse(message)) return;
      appendActivity(message.success ? 'response' : 'error', message.success ? `Command ${message.id} succeeded` : `Command ${message.id} failed`, message.message || '');
      return;
    }
    if (message.type === 'text-response') {
      if (!message.success) appendActivity('error', `Text ${message.id} failed`, message.message || '');
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
  if (sendText(value, value)) {
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
  if (sendText(command, command)) {
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
resetAuthoritativeState();
connect();
