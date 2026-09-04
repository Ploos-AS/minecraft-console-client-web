(() => {
  'use strict';

  const chat = document.getElementById('chat');
  const log = document.getElementById('log');
  const bridgeState = document.getElementById('bridge-state');
  if (!chat || !log || !bridgeState) return;

  const CHAT_WINDOW_MS = 1500;
  const CHAT_CACHE_LIMIT = 64;
  const RAW_HOLD_MS = 180;
  const ERROR_WINDOW_MS = 5000;
  const recentStructured = [];
  const recentErrors = new Map();

  function normalize(value) {
    return String(value || '')
      .replace(/§[0-9A-FK-OR]/gi, '')
      .replace(/\s+/g, ' ')
      .trim();
  }

  function pruneStructured(now = Date.now()) {
    while (recentStructured.length && now - recentStructured[0].at > CHAT_WINDOW_MS) {
      recentStructured.shift();
    }
    while (recentStructured.length > CHAT_CACHE_LIMIT) recentStructured.shift();
  }

  function rememberStructured(row) {
    const body = row.querySelector('p')?.textContent || '';
    const sender = row.querySelector('strong')?.textContent || '';
    const values = new Set([normalize(body)]);
    if (sender) {
      values.add(normalize(`<${sender}> ${body}`));
      values.add(normalize(`${sender}: ${body}`));
      values.add(normalize(`${sender} ${body}`));
    }
    recentStructured.push({ at: Date.now(), values });
    pruneStructured();
  }

  function isRecentStructured(text) {
    const now = Date.now();
    const value = normalize(text);
    pruneStructured(now);
    if (!value) return false;
    return recentStructured.some((entry) => entry.values.has(value));
  }

  function handleChatRow(row) {
    if (!(row instanceof HTMLElement) || !row.classList.contains('chat-row')) return;
    if (row.dataset.m11Restored === 'true') return;
    if (row.classList.contains('public') || row.classList.contains('private')) {
      rememberStructured(row);
      return;
    }
    if (!row.classList.contains('raw')) return;

    const text = row.querySelector('p')?.textContent || '';
    if (isRecentStructured(text)) {
      row.remove();
      return;
    }

    // Hold raw chat very briefly so a matching structured event that arrives
    // immediately afterwards can win without causing a visible duplicate.
    const anchor = row.nextSibling;
    row.remove();
    window.setTimeout(() => {
      if (isRecentStructured(text)) return;
      row.dataset.m11Restored = 'true';
      if (anchor?.parentNode === chat) chat.insertBefore(row, anchor);
      else chat.appendChild(row);
      chat.scrollTop = chat.scrollHeight;
    }, RAW_HOLD_MS);
  }

  const chatObserver = new MutationObserver((records) => {
    for (const record of records) {
      for (const node of record.addedNodes) handleChatRow(node);
    }
  });
  chatObserver.observe(chat, { childList: true });

  function activityKey(row) {
    if (!(row instanceof HTMLElement) || !row.classList.contains('error')) return '';
    const parts = [...row.querySelectorAll('span, small')].map((node) => normalize(node.textContent));
    const summary = parts.join('|');
    if (!summary) return '';
    const connectionFailure = parts.some((part) =>
      part === 'MCC connection error' ||
      part === 'WebAdmin WebSocket error' ||
      part.startsWith('MCC connection error|')
    );
    return connectionFailure ? summary : '';
  }

  function handleActivityRow(row) {
    const key = activityKey(row);
    if (!key) return;
    const now = Date.now();
    const previous = recentErrors.get(key);
    if (previous && now - previous.at <= ERROR_WINDOW_MS && previous.row.isConnected) {
      row.remove();
      previous.at = now;
      previous.count += 1;
      previous.row.dataset.repeatCount = String(previous.count);
      previous.row.title = `Repeated ${previous.count} times in the last ${ERROR_WINDOW_MS / 1000} seconds`;
      return;
    }
    recentErrors.set(key, { at: now, count: 1, row });
    for (const [candidate, entry] of recentErrors) {
      if (now - entry.at > ERROR_WINDOW_MS) recentErrors.delete(candidate);
    }
  }

  const logObserver = new MutationObserver((records) => {
    for (const record of records) {
      for (const node of record.addedNodes) handleActivityRow(node);
    }
  });
  logObserver.observe(log, { childList: true });

  function classifyBridge(text) {
    const value = normalize(text).toLowerCase();
    if (value.includes('connected') && !value.includes('disconnected')) return 'connected';
    if (value.includes('retry')) return 'retrying';
    if (value.includes('connecting')) return 'connecting';
    return 'offline';
  }

  function annotateBridge() {
    const status = classifyBridge(bridgeState.textContent);
    bridgeState.dataset.state = status;
    bridgeState.setAttribute('aria-label', `WebAdmin bridge: ${status}`);
    bridgeState.textContent = `WebAdmin bridge: ${status}`;
  }

  new MutationObserver(annotateBridge).observe(bridgeState, { childList: true, characterData: true, subtree: true });
  annotateBridge();
})();
