// --- XSS Koruması: HTML ve attribute kaçış yardımcıları ---
// Tüm dinamik kullanıcı verilerini HTML içine yerleştirmeden önce escape etmek zorunludur.
function escapeHtml(s) {
  if (s === null || s === undefined) return '';
  return String(s)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;');
}
// Attribute (title="...", value="...") için escape
function escapeAttr(s) { return escapeHtml(s); }
// JavaScript string literal içine gömülecek değeri escape eder (onclick="...('${val}')")
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

// Sayfa yüklendiğinde temel verileri çek ve dinamik alan adını ata
document.addEventListener('DOMContentLoaded', () => {
  // Tema tercihini uygula
  initTheme();

  // Dinamik alan adı ön ekini ata
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
  // Tüm sekmeleri gizle
  const contents = document.querySelectorAll('.tab-content');
  contents.forEach(content => content.classList.remove('active'));

  // Tüm menü butonlarının aktifliğini kaldır
  const buttons = document.querySelectorAll('.nav-item');
  buttons.forEach(btn => btn.classList.remove('active'));

  // Seçilen sekmeyi göster
  const targetContent = document.getElementById(`content-${tabId}`);
  if (targetContent) {
    targetContent.classList.add('active');
  }

  // Seçilen menü butonunu aktif et
  const targetBtn = document.getElementById(`tabBtn-${tabId}`);
  if (targetBtn) {
    targetBtn.classList.add('active');
  }

  // Sayfa başlığını güncelle
  const titleMap = {
    'shortener': 'Link Kısaltıcı',
    'analytics': 'Detaylı Analiz',
    'api': 'API Bağlantısı',
    'users': 'Kullanıcı Yönetimi'
  };
  const titleEl = document.getElementById('pageTitle');
  if (titleEl && titleMap[tabId]) {
    titleEl.innerText = titleMap[tabId];
  }

  // Mobilde menüyü kapat
  const sidebar = document.getElementById('appSidebar');
  const overlay = document.getElementById('sidebarOverlay');
  if (sidebar && sidebar.classList.contains('active')) {
    sidebar.classList.remove('active');
    overlay.classList.remove('active');
  }
}

// Tema Yönetimi (Cihaza göre otomatik + Manuel kontrol)
function initTheme() {
  const cachedTheme = localStorage.getItem('theme');
  const systemTheme = window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
  const theme = cachedTheme || systemTheme;
  
  applyTheme(theme);
  
  // Sistem teması değişikliklerini dinle (kullanıcı manuel seçim yapmadıysa)
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
  
  // Grafik varsa rengini güncellemek için yeniden çizelim
  const detailsWrapper = document.getElementById('analyticsDetailsWrapper');
  if (detailsWrapper && !detailsWrapper.classList.contains('hidden') && clicksChartInstance) {
    const labelEl = document.getElementById('selectedLinkTitle');
    if (labelEl) {
      const code = labelEl.innerText.replace('Analiz Raporu: ', '');
      if (code) loadAnalyticsForSelected(code);
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

// Mobil Sidebar Aç/Kapat
function toggleSidebar() {
  const sidebar = document.getElementById('appSidebar');
  const overlay = document.getElementById('sidebarOverlay');
  if (sidebar) {
    sidebar.classList.toggle('active');
    overlay.classList.toggle('active');
  }
}

// Şifre Göster/Gizle Yardımcısı
function togglePasswordVisibility(inputId, btn) {
  const input = document.getElementById(inputId);
  const icon = btn.querySelector('i');
  if (!input || !icon) return;
  
  if (input.type === 'password') {
    input.type = 'text';
    icon.className = "ph-light ph-eye-closed";
    btn.style.color = "var(--text-primary)";
  } else {
    input.type = 'password';
    icon.className = "ph-light ph-eye";
    btn.style.color = "#9ca3af";
  }
}

// Toast bildirim göstergesi
function showToast(message) {
  const toast = document.getElementById('toast');
  toast.innerText = message;
  toast.classList.add('show');
  setTimeout(() => {
    toast.classList.remove('show');
  }, 2000);
}

// Güvenli Çıkış İşlemi
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

// API Key Göster/Gizle
function toggleApiKey() {
  const input = document.getElementById('apiKeyVal');
  if (input.type === 'password') {
    input.type = 'text';
  } else {
    input.type = 'password';
  }
}

// API Key Kopyala
function copyApiKey() {
  const input = document.getElementById('apiKeyVal');
  navigator.clipboard.writeText(input.value);
  showToast("API Anahtarı kopyalandı!");
}

// API Key Yenileme
async function regenerateApiKey() {
  if (!confirm("API anahtarınızı yenilemek istediğinize emin misiniz? Eski anahtarınızı kullanan servisler artık çalışmayacaktır!")) {
    return;
  }
  try {
    const res = await fetch('/api/v1/users/refresh-token', { method: 'POST' });
    const data = await res.json();
    if (data.success) {
      document.getElementById('apiKeyVal').value = data.data.api_key;
      
      // Dokümanlarda geçen anahtarları da güncelle
      const docKeys = document.querySelectorAll('.doc-api-key');
      docKeys.forEach(el => el.innerText = data.data.api_key);
      
      showToast("API Anahtarı başarıyla yenilendi");
    } else {
      alert("Hata: " + data.error);
    }
  } catch (err) {
    alert("Bağlantı Hatası: API Anahtarı yenilenemedi.");
  }
}

// Linkleri Listeleme ve Arayüzü Besleme
async function fetchLinks() {
  try {
    const res = await fetch('/api/v1/links');
    const data = await res.json();
    
    const tbody = document.getElementById('linksTableBody');
    const analyticsTbody = document.getElementById('analyticsLinksTableBody');

    if (!data.success) {
      const errMsg = `<tr><td colspan="5" style="text-align: center; color: var(--accent-danger);">${escapeHtml(data.error)}</td></tr>`;
      tbody.innerHTML = errMsg;
      if (analyticsTbody) {
        analyticsTbody.innerHTML = `<tr><td colspan="4" style="text-align: center; color: var(--accent-danger);">${escapeHtml(data.error)}</td></tr>`;
      }
      return;
    }

    const links = data.data || [];

    if (links.length === 0) {
      const emptyMsg = `<tr><td colspan="5" style="text-align: center; color: var(--text-muted); padding: 2rem;">Kısaltılmış link bulunmamaktadır.</td></tr>`;
      tbody.innerHTML = emptyMsg;
      if (analyticsTbody) {
        analyticsTbody.innerHTML = `<tr><td colspan="4" style="text-align: center; color: var(--text-muted); padding: 1.5rem;">Kısaltılmış link bulunmamaktadır.</td></tr>`;
      }
      document.getElementById('statsTotalLinks').innerText = '0';
      document.getElementById('statsTotalClicks').innerText = '0';
      return;
    }

    // Toplam Link ve Tıklanma sayısı güncellemeleri
    document.getElementById('statsTotalLinks').innerText = links.length;
    let totalClicks = 0;

    let html = '';
    let analyticsHtml = '';

    links.forEach(link => {
      totalClicks += link.click_count;
      const formattedDate = new Date(link.created_at).toLocaleDateString('tr-TR', {
        year: 'numeric', month: 'short', day: 'numeric'
      });

      // --- Tüm dinamik değerler escape edilir (XSS koruması) ---
      const escShortCode = escapeAttr(link.short_code);
      const escShortUrl = escapeAttr(link.short_url);
      const escOriginalUrlHtml = escapeHtml(link.original_url);
      const escOriginalUrlAttr = escapeAttr(link.original_url);
      const escAlias = escapeJsString(link.custom_alias || '');
      const escShortCodeJs = escapeJsString(link.short_code);
      const escOriginalUrlJs = escapeJsString(link.original_url);

      // 1. Link Kısaltıcı Tablosu Satırı
      html += `
        <tr>
          <td>
            <a href="${escShortUrl}" target="_blank" rel="noopener noreferrer" class="link-url">${escShortCode}</a>
          </td>
          <td>
            <div class="original-url-text" title="${escOriginalUrlAttr}">${escOriginalUrlHtml}</div>
          </td>
          <td style="font-weight: 600; color: var(--text-primary);">${Number(link.click_count)} / ${link.max_clicks > 0 ? Number(link.max_clicks) : 'Sınırsız'}</td>
          <td style="color: var(--text-secondary); font-size: 0.85rem;">${escapeHtml(formattedDate)}</td>
          <td style="text-align: right; white-space: nowrap;">
            <button class="btn btn-secondary btn-small" style="margin-right: 6px; gap:4px;" onclick="openEditModal('${escShortCodeJs}', '${escOriginalUrlJs}', '${escAlias}', ${Number(link.max_clicks)})">
              <i class="ph-light ph-pencil"></i> Düzenle
            </button>
            <button class="btn btn-secondary btn-small" style="margin-right: 6px; gap:4px;" onclick="viewAnalytics('${escShortCodeJs}')">
              <i class="ph-light ph-chart-bar"></i> Analiz
            </button>
            <button class="btn btn-danger btn-small" style="gap:4px;" onclick="deleteLink('${escShortCodeJs}')">
              <i class="ph-light ph-trash"></i> Sil
            </button>
          </td>
        </tr>
      `;

      // 2. Analiz Sekmesi Link Seçim Tablosu Satırı
      analyticsHtml += `
        <tr>
          <td style="font-weight: 600; color: var(--text-primary);">${escShortCode}</td>
          <td>
            <div class="original-url-text" title="${escOriginalUrlAttr}" style="max-width:320px;">${escOriginalUrlHtml}</div>
          </td>
          <td style="font-weight: 600;">${Number(link.click_count)} / ${link.max_clicks > 0 ? Number(link.max_clicks) : 'Sınırsız'}</td>
          <td style="text-align: right;">
            <button class="btn btn-secondary btn-small" style="gap:4px;" onclick="loadAnalyticsForSelected('${escShortCodeJs}')">
              <i class="ph-light ph-eye"></i> Seç ve İncele
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

  alertDiv.innerHTML = '';
  btn.disabled = true;
  btn.innerText = 'Kısaltılıyor...';

  try {
    const res = await fetch('/api/v1/links', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ url: url, custom_alias: alias, max_clicks: maxClicksVal })
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

// --- Düzenleme Modalı Fonksiyonları ---
function openEditModal(shortCode, originalUrl, customAlias, maxClicks) {
  const modal = document.getElementById('editLinkModal');
  if (!modal) return;
  
  document.getElementById('editOldShortCode').value = shortCode;
  document.getElementById('editOriginalUrl').value = originalUrl;
  document.getElementById('editCustomAlias').value = customAlias;
  document.getElementById('editMaxClicks').value = maxClicks;
  
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

  if (alertDiv) alertDiv.innerHTML = '';
  btn.disabled = true;
  btn.innerText = 'Kaydediliyor...';

  try {
    const res = await fetch(`/api/v1/links/${oldCode}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ url: url, custom_alias: alias, max_clicks: maxClicks })
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

// Kullanıcı Yönetimi - Kullanıcıları Listele (Superadmin)
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
    
    // Oturum açmış kullanıcının adını arayüzden oku
    const currentUsername = document.querySelector('.user-name')?.innerText?.trim();

    users.forEach(u => {
      const roleStr = u.role === 'superadmin' ? 'Yönetici' : 'Üye';
      const safeRole = escapeAttr(u.role);
      
      // Superadmin kendisini veya başka bir admini silemez
      const isSelfOrAdmin = u.username === currentUsername || u.role === 'superadmin';
      
      // ID yalnızca sayı; username hem HTML bağlamında hem JS string bağlamında kullanılıyor
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

// Kullanıcı Yönetimi - Kullanıcı Ekle (Superadmin)
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

// Kullanıcı Yönetimi - Kullanıcı Sil (Superadmin)
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

// Tablodaki "Analiz" butonuna tıklandığında tetiklenir
function viewAnalytics(shortCode) {
  // Analiz sekmesine geçiş yap
  switchTab('analytics');
  
  // Analizi yükle
  loadAnalyticsForSelected(shortCode);
}

// Seçilen linke göre analitiği API'den çeker ve grafik/listeleri doldurur
async function loadAnalyticsForSelected(shortCode) {
  const detailsWrapper = document.getElementById('analyticsDetailsWrapper');
  const emptyState = document.getElementById('analyticsEmptyState');

  if (!shortCode) {
    detailsWrapper.classList.add('hidden');
    emptyState.classList.remove('hidden');
    return;
  }

  emptyState.classList.add('hidden');
  detailsWrapper.classList.remove('hidden');

  try {
    const res = await fetch(`/api/v1/analytics/${shortCode}`);
    const data = await res.json();

    if (!data.success) {
      alert("Analitik verileri alınamadı: " + data.error);
      loadAnalyticsForSelected(''); // Sıfırla
      return;
    }

    const stats = data.data;

    // 1. Grafik çizimi (Günlük Tıklanmalar) - DOM reflow gecikmesini önlemek için setTimeout ile çağır
    setTimeout(() => {
      drawClicksChart(stats.daily_clicks || {});
    }, 80);

    // 2. Kırılım listelerini doldur
    populateList('countryList', stats.countries || {});
    populateList('referrerList', stats.referrers || {});
    populateList('browserList', stats.browsers || {});
    populateList('osList', stats.os || {});

    // Sayfa içi kaydırma (Seç ve incele yapınca direkt analiz detaylarına insin)
    detailsWrapper.scrollIntoView({ behavior: 'smooth', block: 'start' });

  } catch (err) {
    alert("Analitik verileri alınırken bağlantı hatası oluştu.");
    loadAnalyticsForSelected('');
  }
}

function populateList(elementId, dataMap) {
  const container = document.getElementById(elementId);
  if (!container) return;
  
  const items = Object.entries(dataMap).sort((a, b) => b[1] - a[1]); // Çoktan aza sırala

  if (items.length === 0) {
    container.innerHTML = `<div class="analytics-list-item"><span style="color:var(--text-muted);">Veri bulunmuyor</span></div>`;
    return;
  }

  const maxVal = items.length > 0 ? items[0][1] : 1;
  let html = '';
  items.forEach(([key, val]) => {
    let label = key;
    // Localhost / loopback ip adresleri için Türkçe ülke adı göster
    if (elementId === 'countryList') {
      if (key === 'localhost' || key === '127.0.0.1' || key === '::1' || key === 'UNKNOWN' || key === '') {
        label = 'Türkiye (Yerel Ağ)';
      }
    }
    const pct = maxVal > 0 ? (val / maxVal) * 100 : 0;
    // Sayısal val; XSS riski yok ama Number() ile emin olalım. label ise tamamen escape edilir.
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

  // Tarihleri sıralı olarak al
  const sortedDays = Object.keys(dailyClicks).sort();
  const counts = sortedDays.map(day => dailyClicks[day]);

  if (sortedDays.length === 0) {
    sortedDays.push(new Date().toLocaleDateString('tr-TR'));
    counts.push(0);
  }

  // Koyu / Açık temaya göre renkleri belirle
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
      
      // Kullanıcı adı alanlarını sayfada dinamik olarak güncelle
      const nameEls = document.querySelectorAll('.user-name');
      nameEls.forEach(el => {
        el.innerText = username;
        el.title = username;
      });
      
      // Avatar harfini güncelle
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
