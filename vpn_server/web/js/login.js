document.getElementById('loginForm').addEventListener('submit', async function(e) {
    e.preventDefault();
    const username = document.getElementById('username').value.trim();
    const password = document.getElementById('password').value;
    const errorDiv = document.getElementById('loginError');
    errorDiv.textContent = '';
    try {
        const resp = await fetch('/api/login', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ username, password })
        });
        if (resp.ok) {
            // 登录成功，跳转到主页面
            window.location.href = 'index.html';
        } else {
            const msg = await resp.text();
            errorDiv.textContent = msg || '登录失败，请检查用户名和密码';
        }
    } catch (err) {
        errorDiv.textContent = '网络错误，请稍后重试';
    }
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
