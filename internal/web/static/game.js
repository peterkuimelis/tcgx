(function() {
  'use strict';

  // ─── State ───
  let ws = null;
  let cardCatalog = {};  // name → CardInfo
  let currentState = null;
  let selectionMode = null;  // { candidates, min, max, selected: Set }

  // ─── DOM refs ───
  const $ = id => document.getElementById(id);

  const lobby = $('lobby');
  const gameDiv = $('game');
  const joinBtn = $('join-btn');
  const lobbyStatus = $('lobby-status');
  const deckSelect = $('deck-select');
  const serverAddr = $('server-addr');

  const oppHP = $('opp-hp');
  const oppHandCount = $('opp-hand-count');
  const oppDeckCount = $('opp-deck-count');
  const oppSH = $('opp-sh-count');
  const oppAgents = $('opp-agents');
  const oppTech = $('opp-tech');
  const oppOS = $('opp-os');

  const yourHP = $('your-hp');
  const yourDeckCount = $('your-deck-count');
  const yourSH = $('your-sh-count');
  const yourAgents = $('your-agents');
  const yourTech = $('your-tech');
  const yourOS = $('your-os');
  const hand = $('hand');

  const turnInfo = $('turn-info');
  const actionPrompt = $('action-prompt');
  const actionButtons = $('action-buttons');
  const eventLogContent = $('event-log-content');

  const tooltip = $('tooltip');
  const tooltipArt = $('tooltip-art');
  const tooltipName = $('tooltip-name');
  const tooltipType = $('tooltip-type');
  const tooltipStats = $('tooltip-stats');
  const tooltipDesc = $('tooltip-desc');

  const gameOverModal = $('game-over-modal');
  const gameOverResult = $('game-over-result');
  const backToLobbyBtn = $('back-to-lobby-btn');

  const oppInfo = $('opp-info');

  // ─── Init ───
  fetchCardCatalog();
  fetchDecks();

  joinBtn.addEventListener('click', joinGame);
  backToLobbyBtn.addEventListener('click', backToLobby);
  document.addEventListener('mousemove', moveTooltip);

  function fetchCardCatalog() {
    fetch('/api/cards')
      .then(r => r.json())
      .then(cards => {
        cards.forEach(c => { cardCatalog[c.name] = c; });
      })
      .catch(() => {});
  }

  function fetchDecks() {
    fetch('/api/decks')
      .then(r => r.json())
      .then(decks => {
        deckSelect.innerHTML = '';
        decks.forEach(d => {
          const opt = document.createElement('option');
          opt.value = d.number;
          opt.textContent = d.number + ': ' + d.name;
          deckSelect.appendChild(opt);
        });
      })
      .catch(() => {});
  }

  // ─── Connection ───

  function joinGame() {
    const addr = serverAddr.value.trim();
    const deckNum = parseInt(deckSelect.value, 10);
    if (!addr) { lobbyStatus.textContent = 'Enter a server address'; return; }

    joinBtn.disabled = true;
    lobbyStatus.textContent = 'Connecting...';

    const wsProto = location.protocol === 'https:' ? 'wss:' : 'ws:';
    ws = new WebSocket(wsProto + '//' + location.host + '/ws');

    ws.onopen = () => {
      ws.send(JSON.stringify({ type: 'connect', addr: addr, deck_number: deckNum }));
      lobbyStatus.textContent = 'Waiting for game to start...';
    };

    ws.onmessage = (ev) => {
      let msg;
      try { msg = JSON.parse(ev.data); } catch { return; }
      if (msg.type === 'error') {
        lobbyStatus.textContent = msg.result || 'Connection failed';
        joinBtn.disabled = false;
        ws.close();
        ws = null;
        return;
      }
      // First real game message → switch to game view
      if (lobby.style.display !== 'none' && !lobby.classList.contains('hidden')) {
        lobby.classList.add('hidden');
        gameDiv.classList.remove('hidden');
      }
      handleServerMessage(msg);
    };

    ws.onclose = () => {
      if (!lobby.classList.contains('hidden')) {
        lobbyStatus.textContent = 'Disconnected';
        joinBtn.disabled = false;
      }
    };

    ws.onerror = () => {
      lobbyStatus.textContent = 'WebSocket error';
      joinBtn.disabled = false;
    };
  }

  function backToLobby() {
    gameOverModal.classList.add('hidden');
    gameDiv.classList.add('hidden');
    lobby.classList.remove('hidden');
    joinBtn.disabled = false;
    lobbyStatus.textContent = '';
    eventLogContent.innerHTML = '';
    currentState = null;
    selectionMode = null;
    if (ws) { ws.close(); ws = null; }
  }

  function send(msg) {
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify(msg));
    }
  }

  // ─── Message Handling ───

  function handleServerMessage(msg) {
    switch (msg.type) {
      case 'notify':
        appendEvent(msg.event);
        break;
      case 'choose_action':
        if (msg.state) renderState(msg.state);
        renderActions(msg.actions);
        break;
      case 'choose_cards':
        if (msg.state) renderState(msg.state);
        renderCardChoice(msg.prompt, msg.candidates, msg.min, msg.max);
        break;
      case 'choose_yes_no':
        if (msg.state) renderState(msg.state);
        renderYesNo(msg.prompt);
        break;
      case 'game_over':
        showGameOver(msg.result, msg.winner);
        break;
    }
  }

  // ─── State Rendering ───

  function renderState(state) {
    currentState = state;
    selectionMode = null;

    const opp = state.opponent;
    const you = state.you;

    // Opponent
    oppHP.textContent = opp.hp;
    oppHandCount.textContent = opp.hand_count;
    oppDeckCount.textContent = opp.deck_count;
    oppSH.textContent = opp.scrapheap_count;

    // Opponent hand visual
    renderOppHandBacks(opp.hand_count);

    // Opponent field
    renderZones(oppAgents, opp.agents, false, 'agent');
    renderZones(oppTech, opp.tech_zone, false, 'tech');
    renderOSZone(oppOS, opp.os, false);

    // You
    yourHP.textContent = you.hp;
    yourDeckCount.textContent = you.deck_count;
    yourSH.textContent = you.scrapheap_count;

    // Your field
    renderZones(yourAgents, you.agents, true, 'agent');
    renderZones(yourTech, you.tech_zone, true, 'tech');
    renderOSZone(yourOS, you.os, true);

    // Hand
    renderHand(you.hand || []);

    // Turn info
    let info = 'Turn ' + state.turn + ' | ' + state.phase;
    info += state.is_your_turn ? ' | Your turn' : " | Opponent's turn";
    turnInfo.textContent = info;

    // Clear actions
    actionPrompt.textContent = '';
    actionButtons.innerHTML = '';
  }

  function renderOppHandBacks(count) {
    // Remove old hand backs
    let existing = oppInfo.querySelector('.opp-hand-display');
    if (existing) existing.remove();
    if (count <= 0) return;
    const container = document.createElement('span');
    container.className = 'opp-hand-display';
    for (let i = 0; i < count; i++) {
      const back = document.createElement('span');
      back.className = 'card-back-mini';
      container.appendChild(back);
    }
    oppInfo.appendChild(container);
  }

  function renderZones(container, zones, isOwner, zoneType) {
    container.innerHTML = '';
    for (let i = 0; i < 5; i++) {
      const z = zones[i];
      const el = createFieldCard(z, isOwner, zoneType);
      container.appendChild(el);
    }
  }

  function renderOSZone(container, os, isOwner) {
    container.innerHTML = '';
    if (!os || os.empty) {
      container.parentElement.classList.add('hidden');
      return;
    }
    container.parentElement.classList.remove('hidden');
    const el = createFieldCard(os, isOwner, 'tech');
    container.appendChild(el);
  }

  function createFieldCard(zone, isOwner, zoneType) {
    const el = document.createElement('div');
    const isTech = zoneType === 'tech';
    el.className = 'card' + (isTech ? ' tech-card' : '');

    if (!zone || zone.empty) {
      el.classList.add('card-empty');
      return el;
    }

    if (zone.face_down) {
      el.classList.add('card-facedown');
      if (isOwner && zone.name) {
        const nameEl = document.createElement('div');
        nameEl.className = 'card-name';
        nameEl.textContent = zone.name;
        el.appendChild(nameEl);
        setupTooltip(el, zone.name);
      } else {
        const nameEl = document.createElement('div');
        nameEl.className = 'card-name';
        nameEl.textContent = 'SET';
        el.appendChild(nameEl);
      }
      return el;
    }

    // Face-up
    if (!isTech && zone.position === 'ATK') el.classList.add('card-atk');
    if (!isTech && zone.position === 'DEF') el.classList.add('card-def');

    // Art background
    if (zone.name) {
      setCardArt(el, zone.name);
    }

    const nameEl = document.createElement('div');
    nameEl.className = 'card-name';
    nameEl.textContent = zone.name || '';
    el.appendChild(nameEl);

    if (!isTech && (zone.atk || zone.def)) {
      const stats = document.createElement('div');
      stats.className = 'card-stats';
      stats.textContent = zone.atk + '/' + zone.def;
      el.appendChild(stats);
    }

    if (zone.name) setupTooltip(el, zone.name);
    return el;
  }

  function renderHand(cards) {
    hand.innerHTML = '';
    cards.forEach((name, idx) => {
      const el = document.createElement('div');
      el.className = 'hand-card';
      el.dataset.index = idx;
      el.dataset.cardName = name;

      setCardArt(el, name);

      const nameEl = document.createElement('div');
      nameEl.className = 'card-name';
      nameEl.textContent = name;
      el.appendChild(nameEl);

      const info = cardCatalog[name];
      if (info && info.cardType === 'Agent') {
        const stats = document.createElement('div');
        stats.className = 'card-stats';
        stats.textContent = info.atk + '/' + info.def;
        el.appendChild(stats);
      }

      setupTooltip(el, name);
      hand.appendChild(el);
    });
  }

  function setCardArt(el, name) {
    const info = cardCatalog[name];
    if (info && info.artPath) {
      el.style.backgroundImage = 'url(' + info.artPath + ')';
      el.classList.add('has-art');
    }
  }

  // ─── Actions ───

  function renderActions(actions) {
    actionPrompt.textContent = 'Choose an action:';
    actionButtons.innerHTML = '';
    actions.forEach(a => {
      const btn = document.createElement('button');
      btn.className = 'action-btn';
      btn.textContent = a.desc;
      btn.addEventListener('click', () => {
        send({ type: 'action', index: a.index });
        actionPrompt.textContent = 'Waiting...';
        actionButtons.innerHTML = '';
      });
      actionButtons.appendChild(btn);
    });
  }

  // ─── Card Choice ───

  function renderCardChoice(prompt, candidates, min, max) {
    selectionMode = { candidates, min, max, selected: new Set() };

    const label = prompt + ' (select ' + min + (max !== min ? '-' + max : '') + ')';
    actionPrompt.textContent = label;
    actionButtons.innerHTML = '';

    // Make candidate cards selectable
    candidates.forEach(c => {
      // Try to find the card in the hand
      const handCards = hand.querySelectorAll('.hand-card');
      let found = false;
      handCards.forEach(el => {
        if (el.dataset.cardName === c.name && !el.classList.contains('selectable')) {
          el.classList.add('selectable');
          el.dataset.candidateIndex = c.index;
          el.addEventListener('click', onCardSelect);
          found = true;
        }
      });

      // If not in hand, add a button
      if (!found) {
        const btn = document.createElement('button');
        btn.className = 'action-btn selectable';
        btn.textContent = c.name + (c.atk || c.def ? ' (' + c.atk + '/' + c.def + ')' : '');
        btn.dataset.candidateIndex = c.index;
        btn.addEventListener('click', () => {
          const idx = parseInt(btn.dataset.candidateIndex, 10);
          toggleSelection(idx);
          btn.classList.toggle('selected', selectionMode.selected.has(idx));
          updateConfirmBtn();
        });
        actionButtons.appendChild(btn);
      }
    });

    // Confirm button
    const confirmBtn = document.createElement('button');
    confirmBtn.className = 'confirm-btn';
    confirmBtn.id = 'confirm-selection';
    confirmBtn.textContent = 'Confirm Selection';
    confirmBtn.disabled = true;
    confirmBtn.addEventListener('click', submitCardSelection);
    actionButtons.appendChild(confirmBtn);
  }

  function onCardSelect(ev) {
    if (!selectionMode) return;
    const el = ev.currentTarget;
    const idx = parseInt(el.dataset.candidateIndex, 10);
    toggleSelection(idx);
    el.classList.toggle('selected', selectionMode.selected.has(idx));
    updateConfirmBtn();
  }

  function toggleSelection(idx) {
    if (!selectionMode) return;
    if (selectionMode.selected.has(idx)) {
      selectionMode.selected.delete(idx);
    } else {
      if (selectionMode.selected.size >= selectionMode.max) return;
      selectionMode.selected.add(idx);
    }
  }

  function updateConfirmBtn() {
    const btn = $('confirm-selection');
    if (!btn || !selectionMode) return;
    const count = selectionMode.selected.size;
    btn.disabled = count < selectionMode.min || count > selectionMode.max;
  }

  function submitCardSelection() {
    if (!selectionMode) return;
    const indices = Array.from(selectionMode.selected);
    send({ type: 'cards', indices: indices });
    selectionMode = null;
    clearSelectable();
    actionPrompt.textContent = 'Waiting...';
    actionButtons.innerHTML = '';
  }

  function clearSelectable() {
    document.querySelectorAll('.selectable').forEach(el => {
      el.classList.remove('selectable', 'selected');
      el.removeEventListener('click', onCardSelect);
    });
  }

  // ─── Yes/No ───

  function renderYesNo(prompt) {
    actionPrompt.textContent = prompt;
    actionButtons.innerHTML = '';

    const yesBtn = document.createElement('button');
    yesBtn.className = 'yes-no-btn yes-btn';
    yesBtn.textContent = 'Yes';
    yesBtn.addEventListener('click', () => {
      send({ type: 'yes_no', answer: true });
      actionPrompt.textContent = 'Waiting...';
      actionButtons.innerHTML = '';
    });

    const noBtn = document.createElement('button');
    noBtn.className = 'yes-no-btn no-btn';
    noBtn.textContent = 'No';
    noBtn.addEventListener('click', () => {
      send({ type: 'yes_no', answer: false });
      actionPrompt.textContent = 'Waiting...';
      actionButtons.innerHTML = '';
    });

    const row = document.createElement('div');
    row.style.display = 'flex';
    row.style.gap = '8px';
    row.appendChild(yesBtn);
    row.appendChild(noBtn);
    actionButtons.appendChild(row);
  }

  // ─── Game Over ───

  function showGameOver(result, winner) {
    gameOverResult.textContent = result;
    gameOverModal.classList.remove('hidden');
  }

  // ─── Event Log ───

  function appendEvent(ev) {
    if (!ev) return;
    const entry = document.createElement('div');
    entry.className = 'log-entry';

    const turnSpan = document.createElement('span');
    turnSpan.className = 'log-turn';
    turnSpan.textContent = 'T' + ev.turn + ' ';

    const phaseSpan = document.createElement('span');
    phaseSpan.className = 'log-phase';
    phaseSpan.textContent = ev.phase || '';

    entry.appendChild(turnSpan);
    entry.appendChild(phaseSpan);
    entry.appendChild(document.createTextNode(' ' + ev.details));
    eventLogContent.appendChild(entry);
    eventLogContent.scrollTop = eventLogContent.scrollHeight;
  }

  // ─── Tooltip ───

  function setupTooltip(el, name) {
    el.addEventListener('mouseenter', () => showTooltip(name));
    el.addEventListener('mouseleave', hideTooltip);
  }

  function showTooltip(name) {
    const info = cardCatalog[name];
    if (!info) {
      // Show minimal tooltip for unknown cards
      tooltipName.textContent = name;
      tooltipType.textContent = '';
      tooltipStats.textContent = '';
      tooltipDesc.textContent = '';
      tooltipArt.classList.remove('loaded');
      tooltip.classList.remove('hidden');
      return;
    }

    tooltipName.textContent = info.name;

    // Type line
    let typeLine = '';
    if (info.cardType === 'Agent') {
      typeLine = 'Level ' + info.level + ' ' + (info.attribute || '') + ' ' + (info.agentType || '') + ' Agent';
      if (info.isEffect) typeLine += ' / Effect';
    } else if (info.cardType === 'Program') {
      typeLine = (info.subtype || 'Normal') + ' Program';
    } else if (info.cardType === 'Trap') {
      typeLine = (info.subtype || 'Normal') + ' Trap';
    }
    tooltipType.textContent = typeLine;

    // Stats
    if (info.cardType === 'Agent') {
      tooltipStats.textContent = 'ATK ' + info.atk + ' / DEF ' + info.def;
    } else {
      tooltipStats.textContent = '';
    }

    tooltipDesc.textContent = info.description || '';

    // Art
    if (info.artPath) {
      tooltipArt.src = info.artPath;
      tooltipArt.classList.add('loaded');
    } else {
      tooltipArt.classList.remove('loaded');
    }

    tooltip.classList.remove('hidden');
  }

  function hideTooltip() {
    tooltip.classList.add('hidden');
  }

  function moveTooltip(ev) {
    if (tooltip.classList.contains('hidden')) return;
    const pad = 15;
    let x = ev.clientX + pad;
    let y = ev.clientY + pad;
    // Keep tooltip on screen
    const rect = tooltip.getBoundingClientRect();
    if (x + rect.width > window.innerWidth) x = ev.clientX - rect.width - pad;
    if (y + rect.height > window.innerHeight) y = ev.clientY - rect.height - pad;
    tooltip.style.left = x + 'px';
    tooltip.style.top = y + 'px';
  }
})();
