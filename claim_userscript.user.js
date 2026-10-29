// ==UserScript==
// @name         War Robots Market Auto Claim Free Item
// @namespace    https://market.my.games/
// @version      1.0.0
// @description  Automatically claims the daily free item (e.g. Common Data Pad Silver) on MY.GAMES War Robots Market
// @author       Antigravity
// @match        https://market.my.games/war_robots*
// @match        https://market.my.games/*/war_robots*
// @grant        none
// @run-at       document-idle
// ==/UserScript==

(function () {
    'use strict';

    console.log('[WR AutoClaimer] Script loaded.');

    const CONFIG = {
        checkIntervalMs: 1500, // Check every 1.5 seconds until loaded
        maxRetries: 20,         // Try up to 30 seconds
        autoReloadHours: 0      // Set to e.g. 12 to auto-reload tab periodically (0 to disable)
    };

    let attempts = 0;
    let claimed = false;

    // Create a floating status toast
    function createToast() {
        if (document.getElementById('wr-autoclaim-toast')) return;
        const toast = document.createElement('div');
        toast.id = 'wr-autoclaim-toast';
        toast.style.position = 'fixed';
        toast.style.bottom = '24px';
        toast.style.right = '24px';
        toast.style.zIndex = '999999';
        toast.style.padding = '12px 20px';
        toast.style.borderRadius = '10px';
        toast.style.fontFamily = 'system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif';
        toast.style.fontSize = '14px';
        toast.style.fontWeight = '600';
        toast.style.boxShadow = '0 8px 24px rgba(0,0,0,0.5)';
        toast.style.transition = 'all 0.3s ease';
        toast.style.backgroundColor = '#1e293b';
        toast.style.color = '#38bdf8';
        toast.style.border = '1px solid #334155';
        toast.innerText = '🤖 WR Auto-Claimer: Initializing...';
        document.body.appendChild(toast);
        return toast;
    }

    function updateToast(text, type = 'info') {
        const toast = document.getElementById('wr-autoclaim-toast') || createToast();
        toast.innerText = text;
        if (type === 'success') {
            toast.style.backgroundColor = '#14532d';
            toast.style.color = '#86efac';
            toast.style.border = '1px solid #22c55e';
        } else if (type === 'warning') {
            toast.style.backgroundColor = '#713f12';
            toast.style.color = '#fde047';
            toast.style.border = '1px solid #eab308';
        } else if (type === 'error') {
            toast.style.backgroundColor = '#7f1d1d';
            toast.style.color = '#fca5a5';
            toast.style.border = '1px solid #ef4444';
        }
    }

    // Helper to find button by text content
    function findFreeButton() {
        const buttons = Array.from(document.querySelectorAll('button'));
        
        // Priority 1: Find button with exact text "Free" or "Gratis"
        for (const btn of buttons) {
            const text = (btn.innerText || btn.textContent || '').trim().toLowerCase();
            if (text === 'free' || text === 'gratis') {
                return btn;
            }
        }

        // Priority 2: Look within cards that have "Common Data Pad" or "Silver"
        const cards = Array.from(document.querySelectorAll('div, section, article')).filter(el => {
            const t = el.innerText || '';
            return t.includes('Common Data Pad') && (t.toLowerCase().includes('free') || t.toLowerCase().includes('gratis'));
        });

        for (const card of cards) {
            const btn = card.querySelector('button');
            if (btn && !btn.disabled) {
                return btn;
            }
        }

        return null;
    }

    // Helper to find confirmation button inside modal
    function findModalConfirmButton() {
        // Ant Design modals, generic dialogs, or right-column product info
        const dialogs = document.querySelectorAll('.ant-modal, [role="dialog"], div[class*="modal"], div[class*="productInfo"]');
        for (const dialog of dialogs) {
            const buttons = Array.from(dialog.querySelectorAll('button'));
            const confirmBtn = buttons.find(b => {
                const text = (b.innerText || b.textContent || '').trim().toLowerCase();
                return text === 'free' || text === 'claim' || text.includes('free') || text.includes('gratis');
            });
            if (confirmBtn && !confirmBtn.disabled) return confirmBtn;
        }
        return null;
    }

    async function attemptClaim() {
        if (claimed) return;
        attempts++;

        const freeBtn = findFreeButton();

        if (freeBtn) {
            console.log('[WR AutoClaimer] Found Free button:', freeBtn);
            updateToast('🎁 Free item found! Clicking Free button...', 'info');
            freeBtn.click();

            // Wait 1 second for modal to appear if needed
            setTimeout(() => {
                const modalBtn = findModalConfirmButton();
                if (modalBtn) {
                    console.log('[WR AutoClaimer] Found confirmation modal button:', modalBtn);
                    updateToast('Confirming claim in modal...', 'info');
                    modalBtn.click();
                }

                claimed = true;
                updateToast('✅ Free item claimed successfully!', 'success');
                console.log('[WR AutoClaimer] Successfully processed claim.');

                // Auto hide toast after 6 seconds
                setTimeout(() => {
                    const toast = document.getElementById('wr-autoclaim-toast');
                    if (toast) toast.style.opacity = '0';
                }, 6000);
            }, 1000);

            clearInterval(timer);
            return;
        }

        // Check if already claimed
        const bodyText = document.body.innerText || '';
        if (bodyText.includes('Common Data Pad') && (bodyText.includes('Claimed') || bodyText.includes('Sudah diklaim'))) {
            claimed = true;
            updateToast('ℹ️ Free item already claimed today.', 'warning');
            clearInterval(timer);
            return;
        }

        updateToast(`🤖 Scanning items... (attempt ${attempts}/${CONFIG.maxRetries})`, 'info');

        if (attempts >= CONFIG.maxRetries) {
            clearInterval(timer);
            updateToast('⚠️ Free button not found. Make sure you are logged in.', 'warning');
        }
    }

    createToast();
    const timer = setInterval(attemptClaim, CONFIG.checkIntervalMs);

    // Optional periodic auto-reload
    if (CONFIG.autoReloadHours > 0) {
        setTimeout(() => {
            window.location.reload();
        }, CONFIG.autoReloadHours * 3600 * 1000);
    }
})();
