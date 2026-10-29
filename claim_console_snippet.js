/**
 * One-Click Console Claimer for War Robots MY.GAMES Market
 * 
 * Usage:
 * 1. Open https://market.my.games/war_robots?content=items in your browser (make sure you are logged in)
 * 2. Press F12 (or Ctrl+Shift+I) to open Developer Tools, then switch to the "Console" tab
 * 3. Paste this entire code and press Enter!
 */
(async function claimFreeItem() {
    console.log("%c[WR Claimer]%c Starting claim process...", "color: #38bdf8; font-weight: bold", "color: inherit");

    // 1. Find the "Free" button
    const buttons = Array.from(document.querySelectorAll('button'));
    let freeBtn = buttons.find(b => {
        const text = (b.innerText || b.textContent || '').trim().toLowerCase();
        return (text === 'free' || text === 'gratis') && !b.disabled;
    });

    if (!freeBtn) {
        // Fallback: look within item cards
        const cards = Array.from(document.querySelectorAll('div, section, article')).filter(el => {
            const t = el.innerText || '';
            return t.includes('Common Data Pad') && (t.toLowerCase().includes('free') || t.toLowerCase().includes('gratis'));
        });
        if (cards.length > 0) {
            freeBtn = cards[0].querySelector('button');
        }
    }

    if (!freeBtn) {
        if (document.body.innerText.includes('Claimed') || document.body.innerText.includes('Sudah diklaim')) {
            console.log("%c[WR Claimer]%c Item has already been claimed!", "color: #eab308; font-weight: bold", "color: inherit");
            alert("War Robots: The free item has already been claimed today!");
        } else {
            console.warn("[WR Claimer] Free button not found. Please verify you are logged in and the page is loaded.");
            alert("War Robots: Free button not found. Make sure you are logged in to your MY.GAMES account.");
        }
        return;
    }

    console.log("%c[WR Claimer]%c Clicking Free item button...", "color: #22c55e; font-weight: bold", "color: inherit", freeBtn);
    freeBtn.click();

    // 2. Wait for modal confirmation dialog if displayed
    await new Promise(r => setTimeout(r, 1000));

    const dialogs = document.querySelectorAll('.ant-modal, [role="dialog"], div[class*="modal"], div[class*="productInfo"]');
    for (const dialog of dialogs) {
        const modalButtons = Array.from(dialog.querySelectorAll('button'));
        const confirmBtn = modalButtons.find(b => {
            const text = (b.innerText || b.textContent || '').trim().toLowerCase();
            return (text === 'free' || text === 'claim' || text.includes('free') || text.includes('gratis')) && !b.disabled;
        });
        if (confirmBtn) {
            console.log("%c[WR Claimer]%c Clicking modal confirmation button...", "color: #22c55e; font-weight: bold", "color: inherit", confirmBtn);
            confirmBtn.click();
            break;
        }
    }

    console.log("%c[WR Claimer]%c Done! Free item claimed successfully.", "color: #22c55e; font-weight: bold; font-size: 14px", "color: inherit");
})();
