(() => {
  'use strict';

  const refreshButton = document.getElementById('refresh-session');
  const reconnectButton = document.getElementById('reconnect-session');
  const actionState = document.getElementById('session-action-state');
  if (!refreshButton || !reconnectButton || !actionState) return;

  let attachedSocket = null;
  let reconnectRequestID = '';

  function ready() {
    return socket?.readyState === WebSocket.OPEN && state?.textContent === 'connected';
  }

  function setActionState(text, busy = false) {
    actionState.textContent = text;
    refreshButton.disabled = busy || !ready();
    reconnectButton.disabled = busy || !ready();
  }

  function syncControls() {
    if (actionState.dataset.busy === 'true') return;
    const enabled = ready();
    refreshButton.disabled = !enabled;
    reconnectButton.disabled = !enabled;
  }

  function setBusy(text) {
    actionState.dataset.busy = 'true';
    setActionState(text, true);
  }

  function clearBusy(text) {
    delete actionState.dataset.busy;
    setActionState(text, false);
    syncControls();
  }

  function handleMessage(event) {
    let message;
    try { message = JSON.parse(event.data); } catch { return; }
    if (message.type !== 'session-action-response' || message.id !== reconnectRequestID) return;
    if (!message.success) {
      clearBusy(message.message || 'Reconnect failed');
      appendActivity('error', 'Reconnect request failed', message.message || '');
      return;
    }
    actionState.textContent = 'Reconnect accepted — waiting for MCC…';
  }

  function attachSocket() {
    if (!socket || socket === attachedSocket) return;
    attachedSocket = socket;
    socket.addEventListener('message', handleMessage);
    socket.addEventListener('open', syncControls);
    socket.addEventListener('close', syncControls);
  }

  refreshButton.addEventListener('click', () => {
    if (!ready()) return;
    setBusy('Refreshing authoritative state…');
    beginHydration();
    appendActivity('system', 'Manual state refresh requested');
    const observer = new MutationObserver(() => {
      if (!hydrationState.textContent.startsWith('Refreshing')) {
        observer.disconnect();
        clearBusy(hydrationState.textContent === 'Authoritative state loaded' ? 'State refreshed' : hydrationState.textContent);
      }
    });
    observer.observe(hydrationState, { childList: true, characterData: true, subtree: true });
  });

  reconnectButton.addEventListener('click', () => {
    if (!ready()) return;
    if (!window.confirm('Reconnect the shared MCC WebSocket session? Pending commands will fail and all WebAdmin tabs will observe the reconnect.')) return;
    reconnectRequestID = `session-${Date.now()}`;
    setBusy('Requesting MCC reconnect…');
    socket.send(JSON.stringify({ type: 'session-action', id: reconnectRequestID, action: 'reconnect' }));
    appendActivity('system', 'Shared MCC reconnect requested', reconnectRequestID);
  });

  const statusObserver = new MutationObserver(() => {
    attachSocket();
    if (state.textContent === 'connected' && actionState.dataset.busy === 'true' && reconnectRequestID) {
      reconnectRequestID = '';
      clearBusy('Reconnect complete');
    } else {
      syncControls();
    }
  });
  statusObserver.observe(state, { childList: true, characterData: true, subtree: true });

  const bridgeObserver = new MutationObserver(() => {
    attachSocket();
    syncControls();
  });
  bridgeObserver.observe(bridgeState, { childList: true, characterData: true, subtree: true });

  attachSocket();
  actionState.textContent = 'Ready';
  syncControls();
})();
