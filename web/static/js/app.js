// --- XSS Koruması: HTML ve attribute kaçış yardımcıları ---
function escapeHtml(s) {
  if (s === null || s === undefined) return '';
  return String(s)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;');
}

function escapeAttr(s) { return escapeHtml(s); }

function escapeJsString(s) {
  if (s === null || s === undefined) return '';
  return String(s)
    .replace(/\\/g, '\\\\')
    .replace(/'/g, "\\'")
    .replace(/"/g, '\\"')
    .replace(/`/g, '\\`')
    .replace(/</g, '\\u003c')
    .replace(/>/g, '\\u003e')
    .replace(/\r/g, '\\r')
    .replace(/\n/g, '\\n')
    .replace(/\u2028/g, '\\u2028')
    .replace(/\u2029/g, '\\u2029');
}

// Filtreleme ve Arama Durumu
let currentFilterStatus = '';
let currentSearchQuery = '';
let searchDebounceTimer = null;
let currentSelectedAnalyticsCode = '';

// Sayfa yüklendiğinde temel verileri çek ve dinamik alan adını ata
document.addEventListener('DOMContentLoaded', () => {
  initTheme();

  const aliasPrefixEl = document.getElementById('aliasDomainPrefix');
  if (aliasPrefixEl) {
    aliasPrefixEl.innerText = window.location.host + '/';
  }

  fetchLinks();
  if (document.getElementById('usersTableBody')) {
    fetchUsers();
  }
});

// Sekme (Tab) Değiştirme Mantığı
function switchTab(tabId) {
  const contents = document.querySelectorAll('.tab-content');
  contents.forEach(content => content.classList.remove('active'));

  const buttons = document.querySelectorAll('.nav-item');
  buttons.forEach(btn => btn.classList.remove('active'));

  const targetContent = document.getElementById(`content-${tabId}`);
  if (targetContent) {
    targetContent.classList.add('active');
  }

  const targetBtn = document.getElementById(`tabBtn-${tabId}`);
  if (targetBtn) {
    targetBtn.classList.add('active');
  }

  const titleMap = {
    'shortener': 'Link Kısaltıcı',
    'analytics': 'Detaylı Analiz',
    'api': 'API & MCP Bağlantısı',
    'users': 'Kullanıcı Yönetimi'
  };
  const titleEl = document.getElementById('pageTitle');
  if (titleEl && titleMap[tabId]) {
    titleEl.innerText = titleMap[tabId];
  }

  const sidebar = document.getElementById('appSidebar');
  const overlay = document.getElementById('sidebarOverlay');
  if (sidebar && sidebar.classList.contains('active')) {
    sidebar.classList.remove('active');
    overlay.classList.remove('active');
  }
}

// Tema Yönetimi
function initTheme() {
  const cachedTheme = localStorage.getItem('theme');
  const systemTheme = window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
  const theme = cachedTheme || systemTheme;
  
  applyTheme(theme);
  
  window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', e => {
    if (!localStorage.getItem('theme')) {
      applyTheme(e.matches ? 'dark' : 'light');
    }
  });
}

function applyTheme(theme) {
  const body = document.body;
  const icon = document.getElementById('themeToggleIcon');
  
  if (theme === 'dark') {
    body.classList.replace('light-theme', 'dark-theme');
    if (icon) icon.className = 'ph-light ph-sun';
  } else {
    body.classList.replace('dark-theme', 'light-theme');
    if (icon) icon.className = 'ph-light ph-moon';
  }
  
  const detailsWrapper = document.getElementById('analyticsDetailsWrapper');
  if (detailsWrapper && !detailsWrapper.classList.contains('hidden') && clicksChartInstance) {
    if (currentSelectedAnalyticsCode) {
      loadAnalyticsForSelected(currentSelectedAnalyticsCode);
    }
  }
}

function toggleTheme() {
  const body = document.body;
  const currentTheme = body.classList.contains('dark-theme') ? 'dark' : 'light';
  const newTheme = currentTheme === 'dark' ? 'light' : 'dark';
  
  localStorage.setItem('theme', newTheme);
  applyTheme(newTheme);
}

function toggleSidebar() {
  const sidebar = document.getElementById('appSidebar');
  const overlay = document.getElementById('sidebarOverlay');
  if (sidebar) {
    sidebar.classList.toggle('active');
    overlay.classList.toggle('active');
  }
}

function togglePasswordVisibility(inputId, btn) {
  const input = document.getElementById(inputId);
  const icon = btn.querySelector('i');
  if (!input || !icon) return;
  
  if (input.type === 'password') {
    input.type = 'text';
    icon.className = "ph-light ph-eye-slash";
    btn.style.color = "var(--text-primary)";
  } else {
    input.type = 'password';
    icon.className = "ph-light ph-eye";
    btn.style.color = "var(--text-muted)";
  }
}

function showToast(message) {
  const toast = document.getElementById('toast');
  toast.innerText = message;
  toast.classList.add('show');
  setTimeout(() => {
    toast.classList.remove('show');
  }, 2200);
}

async function handleLogout() {
  try {
    const res = await fetch('/api/v1/logout', { method: 'POST' });
    const data = await res.json();
    if (data.success) {
      window.location.href = '/login';
    }
  } catch (err) {
    showToast("Çıkış yapılırken hata oluştu");
  }
}

function copyApiKey() {
  const input = document.getElementById('apiKeyField');
  if (input) {
    navigator.clipboard.writeText(input.value);
    showToast("API Anahtarı panoya kopyalandı!");
  }
}

function copyShortUrl(url) {
  navigator.clipboard.writeText(url);
  showToast("Kısa bağlantı kopyalandı!");
}

async function handleRegenerateKey() {
  if (!confirm("API anahtarınızı yenilemek istediğinize emin misiniz? Eski anahtarınızı kullanan AI ajanları veya harici servisler artık erişemeyecektir!")) {
    return;
  }
  try {
    const res = await fetch('/api/v1/users/refresh-token', { method: 'POST' });
    const data = await res.json();
    if (data.success) {
      const newKey = data.data.api_key;
      const keyField = document.getElementById('apiKeyField');
      if (keyField) keyField.value = newKey;
      
      const docKeys = document.querySelectorAll('.doc-api-key');
      docKeys.forEach(el => el.innerText = newKey);
      
      showToast("API Anahtarı başarıyla yenilendi");
    } else {
      alert("Hata: " + data.error);
    }
  } catch (err) {
    alert("Bağlantı Hatası: API Anahtarı yenilenemedi.");
  }
}

// Gelişmiş Seçenekler Aç/Kapa
function toggleAdvancedShortenOptions() {
  const drawer = document.getElementById('advancedShortenOptions');
  const btn = document.getElementById('advOptToggleBtn');
  if (!drawer || !btn) return;

  const isExpanded = drawer.classList.contains('expanded');
  if (isExpanded) {
    drawer.classList.remove('expanded');
    btn.classList.remove('active');
  } else {
    drawer.classList.add('expanded');
    btn.classList.add('active');
    // Drawer açıldığında ilk alana hafif odaklanma
    const expiresInput = document.getElementById('expiresAt');
    if (expiresInput) {
      setTimeout(() => expiresInput.focus(), 150);
    }
  }
}

// Gelişmiş Seçenekler Aktif Rozetini Güncelle
function updateAdvBadge() {
  const expiresAt = document.getElementById('expiresAt')?.value;
  const password = document.getElementById('linkPassword')?.value;
  const badge = document.getElementById('advActiveBadge');
  if (!badge) return;

  let count = 0;
  if (expiresAt && expiresAt.trim() !== '') count++;
  if (password && password.trim() !== '') count++;

  if (count > 0) {
    badge.innerText = `${count} Ayar`;
    badge.classList.remove('hidden');
  } else {
    badge.classList.add('hidden');
  }
}

// Filtreleme Durumu Ayarla
function setFilterStatus(status) {
  currentFilterStatus = status;
  ['all', 'active', 'inactive', 'expired'].forEach(s => {
    const btn = document.getElementById(`filterBtn-${s}`);
    if (btn) btn.classList.remove('active');
  });

  const activeBtnId = status === '' ? 'filterBtn-all' : `filterBtn-${status}`;
  const activeBtn = document.getElementById(activeBtnId);
  if (activeBtn) activeBtn.classList.add('active');

  fetchLinks();
}

// Arama Girişi (Debounced)
function handleSearchInput(val) {
  clearTimeout(searchDebounceTimer);
  searchDebounceTimer = setTimeout(() => {
    currentSearchQuery = val.trim();
    fetchLinks();
  }, 250);
}

// Linkleri Listeleme ve Arayüzü Besleme
async function fetchLinks() {
  try {
    const params = new URLSearchParams();
    if (currentSearchQuery) params.set('search', currentSearchQuery);
    if (currentFilterStatus) params.set('status', currentFilterStatus);

    const res = await fetch(`/api/v1/links?${params.toString()}`);
    const data = await res.json();
    
    const tbody = document.getElementById('linksTableBody');
    const analyticsTbody = document.getElementById('analyticsLinksTableBody');

    if (!data.success) {
      const errMsg = `<tr><td colspan="5" style="text-align: center; color: var(--accent-danger);">${escapeHtml(data.error)}</td></tr>`;
      tbody.innerHTML = errMsg;
      return;
    }

    // PaginatedResponse veya direkt dizi kontrolü
    let links = [];
    if (Array.isArray(data.data)) {
      links = data.data;
    } else if (data.data && Array.isArray(data.data.items)) {
      links = data.data.items;
    }

    if (links.length === 0) {
      const emptyMsg = `<tr><td colspan="5" style="text-align: center; color: var(--text-muted); padding: 2rem;">Kayıtlı bağlantı bulunamadı.</td></tr>`;
      tbody.innerHTML = emptyMsg;
      if (analyticsTbody) {
        analyticsTbody.innerHTML = `<tr><td colspan="4" style="text-align: center; color: var(--text-muted); padding: 1.5rem;">Bağlantı bulunmamaktadır.</td></tr>`;
      }
      return;
    }

    document.getElementById('statsTotalLinks').innerText = data.total_count !== undefined ? data.total_count : links.length;
    let totalClicks = 0;

    let html = '';
    let analyticsHtml = '';

    links.forEach(link => {
      totalClicks += link.click_count;
      const formattedDate = new Date(link.created_at).toLocaleDateString('tr-TR', {
        year: 'numeric', month: 'short', day: 'numeric'
      });

      const escShortCode = escapeAttr(link.short_code);
      const escShortUrl = escapeAttr(link.short_url || `${window.location.origin}/${link.short_code}`);
      const escOriginalUrlHtml = escapeHtml(link.original_url);
      const escOriginalUrlAttr = escapeAttr(link.original_url);
      const escAlias = escapeJsString(link.custom_alias || '');
      const escShortCodeJs = escapeJsString(link.short_code);
      const escOriginalUrlJs = escapeJsString(link.original_url);
      const expiresAtJs = link.expires_at ? escapeJsString(link.expires_at) : '';

      // Durum rozeti tespiti
      const isExpired = link.expires_at && new Date(link.expires_at) < new Date();
      let statusBadge = '';
      if (!link.is_active) {
        statusBadge = `<span class="status-badge inactive"><i class="ph-light ph-pause"></i> Pasif</span>`;
      } else if (isExpired) {
        statusBadge = `<span class="status-badge expired"><i class="ph-light ph-clock"></i> Süresi Doldu</span>`;
      } else {
        statusBadge = `<span class="status-badge active"><i class="ph-light ph-check-circle"></i> Aktif</span>`;
      }

      let passwordBadge = '';
      if (link.has_password) {
        passwordBadge = `<span class="status-badge locked" title="Şifre Korumalı Bağlantı"><i class="ph-light ph-lock"></i> Şifreli</span>`;
      }

      html += `
        <tr>
          <td>
            <div style="display: flex; align-items: center; gap: 8px; flex-wrap: wrap;">
              <a href="${escShortUrl}" target="_blank" rel="noopener noreferrer" class="link-url">${escShortCode}</a>
              ${statusBadge}
              ${passwordBadge}
            </div>
          </td>
          <td>
            <div class="original-url-text" title="${escOriginalUrlAttr}">${escOriginalUrlHtml}</div>
          </td>
          <td style="font-weight: 600; color: var(--text-primary);">
            ${Number(link.click_count)} / ${link.max_clicks > 0 ? Number(link.max_clicks) : 'Sınırsız'}
          </td>
          <td style="color: var(--text-secondary); font-size: 0.85rem;">${escapeHtml(formattedDate)}</td>
          <td style="text-align: right; white-space: nowrap;">
            <button class="table-action-btn" title="Kısa Bağlantıyı Kopyala" onclick="copyShortUrl('${escShortUrl}')">
              <i class="ph-light ph-copy"></i>
            </button>
            <button class="table-action-btn" title="QR Kodu Görüntüle" onclick="openQRModal('${escShortCodeJs}', '${escShortUrl}')">
              <i class="ph-light ph-qr-code"></i>
            </button>
            <button class="table-action-btn ${link.is_active ? 'active-btn' : 'inactive-btn'}" title="${link.is_active ? 'Bağlantıyı Durdur (Pasife Al)' : 'Bağlantıyı Aç (Aktifleştir)'}" onclick="toggleLinkActive('${escShortCodeJs}')">
              <i class="ph-light ${link.is_active ? 'ph-pause' : 'ph-play'}"></i>
            </button>
            <button class="table-action-btn" title="İstatistikleri Sıfırla" onclick="resetLinkStats('${escShortCodeJs}')">
              <i class="ph-light ph-arrows-counter-clockwise"></i>
            </button>
            <button class="table-action-btn" title="Düzenle" onclick="openEditModal('${escShortCodeJs}', '${escOriginalUrlJs}', '${escAlias}', ${Number(link.max_clicks)}, '${expiresAtJs}', ${link.is_active})">
              <i class="ph-light ph-pencil"></i>
            </button>
            <button class="table-action-btn" title="Sil" style="color: #ef4444;" onclick="deleteLink('${escShortCodeJs}')">
              <i class="ph-light ph-trash"></i>
            </button>
          </td>
        </tr>
      `;

      analyticsHtml += `
        <tr>
          <td style="font-weight: 600; color: var(--text-primary);">
            ${escShortCode} ${statusBadge}
          </td>
          <td>
            <div class="original-url-text" title="${escOriginalUrlAttr}" style="max-width:280px;">${escOriginalUrlHtml}</div>
          </td>
          <td style="font-weight: 600;">${Number(link.click_count)} / ${link.max_clicks > 0 ? Number(link.max_clicks) : 'Sınırsız'}</td>
          <td style="text-align: right;">
            <button class="btn btn-secondary btn-small" style="gap:4px;" onclick="loadAnalyticsForSelected('${escShortCodeJs}')">
              <i class="ph-light ph-eye"></i> İncele
            </button>
          </td>
        </tr>
      `;
    });

    tbody.innerHTML = html;
    if (analyticsTbody) {
      analyticsTbody.innerHTML = analyticsHtml;
    }
    document.getElementById('statsTotalClicks').innerText = totalClicks;

  } catch (err) {
    console.error(err);
    document.getElementById('linksTableBody').innerHTML = `<tr><td colspan="5" style="text-align: center; color: var(--accent-danger); padding: 2rem;">Bağlantı hatası oluştu.</td></tr>`;
  }
}

// Link Kısaltma
async function handleShorten(e) {
  e.preventDefault();
  const alertDiv = document.getElementById('shortenAlert');
  const btn = document.getElementById('shortenBtn');
  const url = document.getElementById('originalUrl').value;
  const alias = document.getElementById('customAlias').value;
  const maxClicksVal = parseInt(document.getElementById('maxClicks').value) || 0;
  const expiresAtVal = document.getElementById('expiresAt').value;
  const passwordVal = document.getElementById('linkPassword').value;

  alertDiv.innerHTML = '';
  btn.disabled = true;
  btn.innerText = 'Kısaltılıyor...';

  const payload = {
    url: url,
    custom_alias: alias,
    max_clicks: maxClicksVal
  };
  if (expiresAtVal) payload.expires_at = expiresAtVal;
  if (passwordVal) payload.password = passwordVal;

  try {
    const res = await fetch('/api/v1/links', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload)
    });
    
    const data = await res.json();
    if (data.success) {
      const shortUrl = escapeAttr(data.data.short_url);
      const shortUrlTxt = escapeHtml(data.data.short_url);
      alertDiv.innerHTML = `
        <div class="alert alert-success" style="margin-bottom:1rem;">
          Link başarıyla kısaltıldı: <a href="${shortUrl}" target="_blank" rel="noopener noreferrer" style="color:var(--text-primary); font-weight:700; text-decoration:underline; margin-left:5px;">${shortUrlTxt}</a>
        </div>
      `;
      document.getElementById('originalUrl').value = '';
      document.getElementById('customAlias').value = '';
      document.getElementById('maxClicks').value = '0';
      document.getElementById('expiresAt').value = '';
      document.getElementById('linkPassword').value = '';
      updateAdvBadge();
      fetchLinks();
    } else {
      alertDiv.innerHTML = `<div class="alert alert-danger" style="margin-bottom:1rem;">${escapeHtml(data.error)}</div>`;
    }
  } catch (err) {
    alertDiv.innerHTML = `<div class="alert alert-danger" style="margin-bottom:1rem;">Kısaltma esnasında bir bağlantı hatası oluştu.</div>`;
  } finally {
    btn.disabled = false;
    btn.innerText = 'Kısalt';
  }
}

// Aktif/Pasif Toggle
async function toggleLinkActive(shortCode) {
  try {
    const res = await fetch(`/api/v1/links/${encodeURIComponent(shortCode)}/toggle`, {
      method: 'PATCH'
    });
    const data = await res.json();
    if (data.success) {
      showToast(data.data.message);
      fetchLinks();
    } else {
      alert("Hata: " + data.error);
    }
  } catch (err) {
    alert("Bağlantı hatası: Durum güncellenemedi.");
  }
}

// İstatistik Sıfırlama
async function resetLinkStats(shortCode) {
  if (!confirm(`'${shortCode}' linkinin tüm tıklama ve analitik verilerini sıfırlamak istediğinize emin misiniz? Bu işlem geri alınamaz!`)) {
    return;
  }
  try {
    const res = await fetch(`/api/v1/links/${encodeURIComponent(shortCode)}/reset-stats`, {
      method: 'POST'
    });
    const data = await res.json();
    if (data.success) {
      showToast("İstatistikler başarıyla sıfırlandı!");
      fetchLinks();
      if (currentSelectedAnalyticsCode === shortCode) {
        loadAnalyticsForSelected(shortCode);
      }
    } else {
      alert("Hata: " + data.error);
    }
  } catch (err) {
    alert("Bağlantı hatası: İstatistikler sıfırlanamadı.");
  }
}

// Analiz sekmesinden mevcut seçili linki sıfırla
function handleResetCurrentStats() {
  if (currentSelectedAnalyticsCode) {
    resetLinkStats(currentSelectedAnalyticsCode);
  }
}

// --- QR Kod Modalı ---
function openQRModal(shortCode, shortURL) {
  const modal = document.getElementById('qrModal');
  const img = document.getElementById('qrModalImage');
  const linkText = document.getElementById('qrModalLinkText');
  const downloadBtn = document.getElementById('qrDownloadBtn');
  const title = document.getElementById('qrModalTitle');

  if (!modal) return;

  const qrSrc = `/api/v1/links/${encodeURIComponent(shortCode)}/qr?size=300`;
  const downloadSrc = `/api/v1/links/${encodeURIComponent(shortCode)}/qr?size=512`;

  title.innerText = `${shortCode} - QR Kod`;
  img.src = qrSrc;
  linkText.innerText = shortURL;
  downloadBtn.href = downloadSrc;
  downloadBtn.download = `qr-${shortCode}.png`;

  modal.classList.add('active');
}

function closeQRModal() {
  const modal = document.getElementById('qrModal');
  if (modal) modal.classList.remove('active');
}

// --- Düzenleme Modalı Fonksiyonları ---
function openEditModal(shortCode, originalUrl, customAlias, maxClicks, expiresAt, isActive) {
  const modal = document.getElementById('editLinkModal');
  if (!modal) return;
  
  document.getElementById('editOldShortCode').value = shortCode;
  document.getElementById('editOriginalUrl').value = originalUrl;
  document.getElementById('editCustomAlias').value = customAlias;
  document.getElementById('editMaxClicks').value = maxClicks;
  document.getElementById('editPassword').value = '';
  document.getElementById('editIsActive').checked = isActive !== false;

  const expField = document.getElementById('editExpiresAt');
  if (expField) {
    if (expiresAt) {
      const d = new Date(expiresAt);
      if (!isNaN(d.getTime())) {
        const localISO = new Date(d.getTime() - d.getTimezoneOffset() * 60000).toISOString().slice(0, 16);
        expField.value = localISO;
      } else {
        expField.value = '';
      }
    } else {
      expField.value = '';
    }
  }
  
  const editPrefix = document.getElementById('editAliasDomainPrefix');
  if (editPrefix) {
    editPrefix.innerText = window.location.host + '/';
  }
  
  modal.classList.add('active');
}

function closeEditModal() {
  const modal = document.getElementById('editLinkModal');
  if (modal) {
    modal.classList.remove('active');
  }
  const alert = document.getElementById('editAlert');
  if (alert) {
    alert.innerHTML = '';
  }
}

async function handleUpdateLink(e) {
  e.preventDefault();
  const alertDiv = document.getElementById('editAlert');
  const btn = document.getElementById('editSaveBtn');
  const oldCode = document.getElementById('editOldShortCode').value;
  const url = document.getElementById('editOriginalUrl').value;
  const alias = document.getElementById('editCustomAlias').value;
  const maxClicks = parseInt(document.getElementById('editMaxClicks').value) || 0;
  const expiresAt = document.getElementById('editExpiresAt').value;
  const password = document.getElementById('editPassword').value;
  const isActive = document.getElementById('editIsActive').checked;

  if (alertDiv) alertDiv.innerHTML = '';
  btn.disabled = true;
  btn.innerText = 'Kaydediliyor...';

  const payload = {
    url: url,
    custom_alias: alias,
    max_clicks: maxClicks,
    is_active: isActive
  };
  if (expiresAt) {
    payload.expires_at = expiresAt;
  } else {
    payload.expires_at = "clear";
  }
  if (password) {
    payload.password = password;
  }

  try {
    const res = await fetch(`/api/v1/links/${encodeURIComponent(oldCode)}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload)
    });
    
    const data = await res.json();
    if (data.success) {
      showToast("Link başarıyla güncellendi!");
      closeEditModal();
      fetchLinks();
    } else {
      if (alertDiv) {
        alertDiv.innerHTML = `<div class="alert alert-danger" style="margin-bottom:1rem;">${escapeHtml(data.error)}</div>`;
      }
    }
  } catch (err) {
    if (alertDiv) {
      alertDiv.innerHTML = `<div class="alert alert-danger" style="margin-bottom:1rem;">Bağlantı hatası oluştu.</div>`;
    }
  } finally {
    btn.disabled = false;
    btn.innerText = 'Değişiklikleri Kaydet';
  }
}

// Link Silme
async function deleteLink(shortCode) {
  if (!confirm(`'${shortCode}' kodlu kısaltılmış linki silmek istediğinize emin misiniz?`)) {
    return;
  }
  try {
    const res = await fetch(`/api/v1/links/${encodeURIComponent(shortCode)}`, { method: 'DELETE' });
    const data = await res.json();
    if (data.success) {
      showToast("Link başarıyla silindi");
      fetchLinks();
    } else {
      alert("Hata: " + data.error);
    }
  } catch (err) {
    alert("Bağlantı hatası: Link silinemedi.");
  }
}

// Kullanıcı Yönetimi
async function fetchUsers() {
  try {
    const res = await fetch('/api/v1/admin/users');
    const data = await res.json();
    const tbody = document.getElementById('usersTableBody');

    if (!data.success) {
      tbody.innerHTML = `<tr><td colspan="3" style="text-align: center; color: var(--accent-danger);">${escapeHtml(data.error)}</td></tr>`;
      return;
    }

    const users = data.data || [];
    let html = '';
    const currentUsername = document.querySelector('.user-name')?.innerText?.trim();

    users.forEach(u => {
      const roleStr = u.role === 'superadmin' ? 'Yönetici' : 'Üye';
      const safeRole = escapeAttr(u.role);
      const isSelfOrAdmin = u.username === currentUsername || u.role === 'superadmin';
      const safeId = Number(u.id);
      const usernameHtml = escapeHtml(u.username);
      const usernameJs = escapeJsString(u.username);

      const actionCell = isSelfOrAdmin
        ? `<span style="color: var(--text-muted); font-size: 0.8rem;">-</span>`
        : `<button class="btn btn-danger btn-small" style="gap:4px;" onclick="deleteUser(${safeId}, '${usernameJs}')">
            <i class="ph-light ph-trash"></i> Sil
           </button>`;

      html += `
        <tr>
          <td style="font-weight: 500;">${usernameHtml}</td>
          <td>
            <span class="user-role-badge ${safeRole}">${escapeHtml(roleStr)}</span>
          </td>
          <td style="text-align: right;">
            ${actionCell}
          </td>
        </tr>
      `;
    });
    tbody.innerHTML = html;
  } catch (err) {
    console.error(err);
  }
}

async function handleCreateUser(e) {
  e.preventDefault();
  const alertDiv = document.getElementById('userAlert');
  const btn = document.getElementById('createUserBtn');
  const username = document.getElementById('newUsername').value;
  const password = document.getElementById('newPassword').value;

  alertDiv.innerHTML = '';
  btn.disabled = true;

  try {
    const res = await fetch('/api/v1/admin/users', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username, password })
    });
    const data = await res.json();
    if (data.success) {
      alertDiv.innerHTML = `<div class="alert alert-success" style="margin-bottom:1rem;">Üye başarıyla eklendi.</div>`;
      document.getElementById('newUsername').value = '';
      document.getElementById('newPassword').value = '';
      fetchUsers();
    } else {
      alertDiv.innerHTML = `<div class="alert alert-danger" style="margin-bottom:1rem;">${escapeHtml(data.error)}</div>`;
    }
  } catch (err) {
    alertDiv.innerHTML = `<div class="alert alert-danger" style="margin-bottom:1rem;">Üye eklenirken hata oluştu.</div>`;
  } finally {
    btn.disabled = false;
  }
}

async function deleteUser(userID, username) {
  if (!confirm(`'${username}' isimli üyeyi sistemden kalıcı olarak silmek istediğinize emin misiniz?`)) {
    return;
  }
  try {
    const res = await fetch(`/api/v1/admin/users/${Number(userID)}`, { method: 'DELETE' });
    const data = await res.json();
    if (data.success) {
      showToast("Kullanıcı başarıyla silindi");
      fetchUsers();
    } else {
      alert("Hata: " + data.error);
    }
  } catch (err) {
    alert("Bağlantı hatası: Kullanıcı silinemedi.");
  }
}

// --- Detay Analitik Sayfa İşlemleri ---
let clicksChartInstance = null;

function viewAnalytics(shortCode) {
  switchTab('analytics');
  loadAnalyticsForSelected(shortCode);
}

async function loadAnalyticsForSelected(shortCode) {
  const detailsWrapper = document.getElementById('analyticsDetailsWrapper');
  const emptyState = document.getElementById('analyticsEmptyState');
  const titleEl = document.getElementById('selectedLinkTitle');

  if (!shortCode) {
    detailsWrapper.classList.add('hidden');
    emptyState.classList.remove('hidden');
    currentSelectedAnalyticsCode = '';
    return;
  }

  currentSelectedAnalyticsCode = shortCode;
  emptyState.classList.add('hidden');
  detailsWrapper.classList.remove('hidden');
  if (titleEl) {
    titleEl.innerText = `Analiz Raporu: ${shortCode}`;
  }

  try {
    const res = await fetch(`/api/v1/analytics/${encodeURIComponent(shortCode)}`);
    const data = await res.json();

    if (!data.success) {
      alert("Analitik verileri alınamadı: " + data.error);
      loadAnalyticsForSelected('');
      return;
    }

    const stats = data.data;

    setTimeout(() => {
      drawClicksChart(stats.daily_clicks || {});
    }, 80);

    populateList('countryList', stats.countries || {});
    populateList('referrerList', stats.referrers || {});
    populateList('browserList', stats.browsers || {});
    populateList('osList', stats.os || {});

    detailsWrapper.scrollIntoView({ behavior: 'smooth', block: 'start' });

  } catch (err) {
    alert("Analitik verileri alınırken bağlantı hatası oluştu.");
    loadAnalyticsForSelected('');
  }
}

function populateList(elementId, dataMap) {
  const container = document.getElementById(elementId);
  if (!container) return;
  
  const items = Object.entries(dataMap).sort((a, b) => b[1] - a[1]);

  if (items.length === 0) {
    container.innerHTML = `<div class="analytics-list-item"><span style="color:var(--text-muted);">Veri bulunmuyor</span></div>`;
    return;
  }

  const maxVal = items.length > 0 ? items[0][1] : 1;
  let html = '';
  items.forEach(([key, val]) => {
    let label = key;
    if (elementId === 'countryList') {
      if (key === 'localhost' || key === '127.0.0.1' || key === '::1' || key === 'UNKNOWN' || key === '') {
        label = 'Türkiye (Yerel Ağ)';
      }
    }
    const pct = maxVal > 0 ? (val / maxVal) * 100 : 0;
    const safeLabelHtml = escapeHtml(label);
    const safeLabelAttr = escapeAttr(label);
    const safeVal = Number(val);
    html += `
      <div class="analytics-list-item">
        <div class="analytics-progress-bar" style="width: ${pct}%"></div>
        <span style="font-weight: 500; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; max-width:80%;" title="${safeLabelAttr}">${safeLabelHtml}</span>
        <span style="font-weight:600; color:var(--text-primary);">${safeVal}</span>
      </div>
    `;
  });
  container.innerHTML = html;
}

function drawClicksChart(dailyClicks) {
  const ctx = document.getElementById('clicksChart').getContext('2d');
  
  if (clicksChartInstance) {
    clicksChartInstance.destroy();
  }

  const sortedDays = Object.keys(dailyClicks).sort();
  const counts = sortedDays.map(day => dailyClicks[day]);

  if (sortedDays.length === 0) {
    sortedDays.push(new Date().toLocaleDateString('tr-TR'));
    counts.push(0);
  }

  const isDark = document.body.classList.contains('dark-theme');
  const gridColor = isDark ? 'rgba(255, 255, 255, 0.05)' : 'rgba(0, 0, 0, 0.03)';
  const tickColor = isDark ? '#a1a1aa' : '#475569';
  const lineColor = isDark ? '#f4f4f5' : '#0f172a';
  const areaColor = isDark ? 'rgba(244, 244, 245, 0.04)' : 'rgba(15, 23, 42, 0.035)';

  clicksChartInstance = new Chart(ctx, {
    type: 'line',
    data: {
      labels: sortedDays.map(d => {
        const parts = d.split('-');
        if (parts.length === 3) return `${parts[2]}/${parts[1]}`;
        return d;
      }),
      datasets: [{
        label: 'Tıklanmalar',
        data: counts,
        borderColor: lineColor,
        backgroundColor: areaColor,
        fill: true,
        tension: 0.25,
        borderWidth: 2,
        pointBackgroundColor: lineColor,
        pointRadius: 4
      }]
    },
    options: {
      responsive: true,
      maintainAspectRatio: false,
      plugins: {
        legend: { display: false }
      },
      scales: {
        y: {
          grid: { color: gridColor },
          ticks: { color: tickColor, precision: 0, font: { family: 'Outfit', size: 11 } }
        },
        x: {
          grid: { display: false },
          ticks: { color: tickColor, font: { family: 'Outfit', size: 11 } }
        }
      }
    }
  });
}

// --- Profil Ayarları (Hesap Yönetimi) ---
function openProfileModal() {
  const modal = document.getElementById('profileModal');
  if (modal) {
    modal.classList.add('active');
  }
}

function closeProfileModal() {
  const modal = document.getElementById('profileModal');
  if (modal) {
    modal.classList.remove('active');
  }
  const alert = document.getElementById('profileAlert');
  if (alert) {
    alert.innerHTML = '';
  }
  const pwdField = document.getElementById('profilePassword');
  if (pwdField) {
    pwdField.value = '';
  }
}

async function handleUpdateProfile(e) {
  e.preventDefault();
  const alertDiv = document.getElementById('profileAlert');
  const btn = document.getElementById('profileSaveBtn');
  const username = document.getElementById('profileUsername').value;
  const password = document.getElementById('profilePassword').value;

  alertDiv.innerHTML = '';
  btn.disabled = true;
  btn.innerText = 'Kaydediliyor...';

  try {
    const res = await fetch('/api/v1/users/profile', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username, password })
    });
    
    const data = await res.json();
    if (data.success) {
      showToast("Profiliniz başarıyla güncellendi!");
      
      const nameEls = document.querySelectorAll('.user-name');
      nameEls.forEach(el => {
        el.innerText = username;
        el.title = username;
      });
      
      const avatarEl = document.getElementById('sidebarUserAvatar');
      if (avatarEl && username) {
        avatarEl.innerText = username.charAt(0).toUpperCase();
      }
      
      closeProfileModal();
    } else {
      alertDiv.innerHTML = `<div class="alert alert-danger" style="margin-bottom:1rem;">${escapeHtml(data.error)}</div>`;
    }
  } catch (err) {
    alertDiv.innerHTML = `<div class="alert alert-danger" style="margin-bottom:1rem;">Bağlantı hatası oluştu.</div>`;
  } finally {
    btn.disabled = false;
    btn.innerText = 'Kaydet';
  }
}
