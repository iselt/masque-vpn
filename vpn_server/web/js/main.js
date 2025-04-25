document.addEventListener('DOMContentLoaded', function() {
    // 检查CA状态
    fetch('/api/ca_status').then(async resp => {
        if (resp.status === 401) {
            window.location.href = 'login.html';
            return;
        }
        if (!resp.ok) throw new Error('网络错误');
        const data = await resp.json();
        if (!data.exists) {
            document.getElementById('caModal').style.display = 'flex';
        }
    }).catch(() => {
        document.getElementById('caStatusMsg').textContent = '无法检测CA状态，请检查网络或后端服务。';
    });

    // 生成CA按钮
    document.getElementById('genCaBtn').onclick = function() {
        this.disabled = true;
        this.textContent = '生成中...';
        fetch('/api/gen_ca_server', { method: 'POST' }).then(async resp => {
            if (resp.status === 401) {
                window.location.href = 'login.html';
                return;
            }
            if (resp.ok) {
                document.getElementById('caModal').style.display = 'none';
                document.getElementById('caStatusMsg').textContent = 'CA证书已生成。';
            } else {
                const msg = await resp.text();
                alert('生成失败：' + msg);
                this.disabled = false;
                this.textContent = '生成CA';
            }
        }).catch(() => {
            alert('网络错误，生成失败');
            this.disabled = false;
            this.textContent = '生成CA';
        });
    };

    // 客户端管理区交互
let highlightClientId = null;
let highlightTimer = null;
function loadClients(newId) {
    fetch('/api/clients').then(resp => {
        if (resp.status === 401) {
            window.location.href = 'login.html';
            return Promise.reject();
        }
        return resp.json();
    }).then(list => {
        const tbody = document.getElementById('clientTableBody');
        tbody.innerHTML = '';
        if (!list.length) {
            tbody.innerHTML = '<tr><td colspan="4">暂无客户端</td></tr>';
            return;
        }
        for (const c of list) {
            const tr = document.createElement('tr');
            tr.innerHTML = `
                <td>${c.client_id}</td>
                <td>${c.created_at ? c.created_at.replace('T',' ').replace(/\..+/, '') : ''}</td>
                <td>${c.online ? '<span class="client-online">在线</span>' : '<span class="client-offline">离线</span>'}</td>
                <td>
                    <button class="download-btn" data-id="${c.client_id}">下载配置</button><br>
                    <button class="delete-btn" data-id="${c.client_id}">删除</button>
                </td>
            `;
            if ((newId && c.client_id === newId) || (highlightClientId && c.client_id === highlightClientId)) {
                tr.classList.add('client-highlight');
            }
            tbody.appendChild(tr);
        }
    });
}

// ========== 新客户端弹窗生成与记忆 ==========
const genClientModal = document.getElementById('genClientModal');
const genClientForm = document.getElementById('genClientForm');
const cancelGenClient = document.getElementById('cancelGenClient');
const addBtn = document.getElementById('addClientBtn');

// 记忆上次输入
function getLastGenClientConfig() {
    try {
        return JSON.parse(localStorage.getItem('masque-last-client-config') || '{}');
    } catch { return {}; }
}
function setLastGenClientConfig(cfg) {
    localStorage.setItem('masque-last-client-config', JSON.stringify(cfg));
}

addBtn.onclick = () => {
    // 自动填充上次输入
    const last = getLastGenClientConfig();
    document.getElementById('gen_server_addr').value = last.server_addr || '';
    document.getElementById('gen_server_name').value = last.server_name || '';
    document.getElementById('gen_mtu').value = last.mtu || '1413';
    document.getElementById('gen_tun_name').value = last.tun_name || '';
    document.getElementById('genClientResult').textContent = '';
    genClientModal.style.display = 'flex';
};

cancelGenClient.onclick = function() {
    genClientModal.style.display = 'none';
};

genClientForm.onsubmit = function(e) {
    e.preventDefault();
    const server_addr = document.getElementById('gen_server_addr').value.trim();
    const server_name = document.getElementById('gen_server_name').value.trim();
    const mtu = document.getElementById('gen_mtu').value.trim();
    const tun_name = document.getElementById('gen_tun_name').value.trim();
    // 记忆本次输入
    setLastGenClientConfig({ server_addr, server_name, mtu, tun_name });
    // 参数校验
    if (!server_addr || !server_name || !mtu) {
        document.getElementById('genClientResult').textContent = '请填写完整信息';
        return;
    }
    genClientForm.querySelector('button[type="submit"]').disabled = true;
    document.getElementById('genClientResult').textContent = '生成中...';
    let params = `server_addr=${encodeURIComponent(server_addr)}&server_name=${encodeURIComponent(server_name)}&mtu=${encodeURIComponent(mtu)}`;
    if (tun_name) params += `&tun_name=${encodeURIComponent(tun_name)}`;
    fetch('/api/gen_client?' + params).then(resp => {
        if (resp.status === 401) {
            window.location.href = 'login.html';
            return Promise.reject();
        }
        return resp.json();
    }).then(res => {
        genClientForm.querySelector('button[type="submit"]').disabled = false;
        if (res.client_id) {
            genClientModal.style.display = 'none';
            highlightClientId = res.client_id;
            loadClients(res.client_id);
            window.open(`/api/download_client?id=${res.client_id}`);
            if (highlightTimer) clearTimeout(highlightTimer);
            highlightTimer = setTimeout(() => {
                highlightClientId = null;
                loadClients();
            }, 3000);
        } else {
            document.getElementById('genClientResult').textContent = '生成失败';
        }
    }).catch(()=>{
        genClientForm.querySelector('button[type="submit"]').disabled = false;
        document.getElementById('genClientResult').textContent = '生成失败';
    });
};

// 下载/删除操作
const clientTable = document.getElementById('clientTableBody');
clientTable.onclick = function(e) {
    if (e.target.classList.contains('download-btn')) {
        const id = e.target.getAttribute('data-id');
        window.open(`/api/download_client?id=${id}`);
    } else if (e.target.classList.contains('delete-btn')) {
        const id = e.target.getAttribute('data-id');
        // 修改确认提示信息
        if (confirm(`确定要删除客户端 ${id} 吗？删除后对应的客户端将无法连接。`)) {
            fetch(`/api/delete_client?id=${id}`).then(resp => {
                if (resp.status === 401) {
                    window.location.href = 'login.html';
                    return;
                }
                if (resp.ok) loadClients();
                else alert('删除失败');
            });
        }
    }
};

    // 页面加载时自动加载客户端列表
    loadClients();
    // 每5秒轮询一次客户端状态
    setInterval(loadClients, 5000);
});

// 主题切换逻辑
(function(){
    const root = document.documentElement;
    const btn = document.getElementById('themeToggle');
    function setTheme(theme) {
        root.setAttribute('data-theme', theme);
        btn.textContent = theme === 'dark' ? '☀️' : '🌙';
        localStorage.setItem('masque-theme', theme);
    }
    function toggleTheme() {
        setTheme(root.getAttribute('data-theme') === 'dark' ? 'light' : 'dark');
    }
    btn.addEventListener('click', toggleTheme);
    // 初始化主题
    const saved = localStorage.getItem('masque-theme');
    if(saved === 'dark' || (saved !== 'light' && window.matchMedia('(prefers-color-scheme: dark)').matches)) {
        setTheme('dark');
    } else {
        setTheme('light');
    }
})();
