// DOM元素
const statusContainer = document.getElementById('status-container');
const loginForm = document.getElementById('login-form');
const codeForm = document.getElementById('code-form');
const successContainer = document.getElementById('success-container');
const errorContainer = document.getElementById('error-container');
const errorText = document.getElementById('error-text');

// 表单元素
const userForm = document.getElementById('user-form');
const codeSubmitForm = document.getElementById('code-submit-form');

// API基础URL
const API_BASE = window.location.origin;

// 显示/隐藏元素的辅助函数
function showElement(element) {
    element.classList.remove('hidden');
    element.classList.add('fade-in');
}

function hideElement(element) {
    element.classList.add('hidden');
    element.classList.remove('fade-in');
}

// 显示错误信息
function showError(message) {
    errorText.textContent = message;
    hideAllContainers();
    showElement(errorContainer);
}

// 隐藏所有容器
function hideAllContainers() {
    [statusContainer, loginForm, codeForm, successContainer, errorContainer].forEach(hideElement);
}

// 检查登录状态
async function checkStatus() {
    hideAllContainers();
    showElement(statusContainer);

    try {
        const response = await fetch(`${API_BASE}/tgad/login/status`);
        const data = await response.json();

        if (data.rtn === 0) {
            switch (data.status) {
                case 0: // 未登录
                    showLoginForm();
                    break;
                case 1: // 登录中
                    showCodeForm();
                    break;
                case 2: // 登录成功
                    showSuccess(data);
                    break;
                case 3: // 登录失败
                    showError('登录失败，请重新尝试');
                    break;
                default:
                    showLoginForm();
            }
        } else {
            showError(`状态检查失败: ${data.msg}`);
        }
    } catch (error) {
        showError(`网络错误: ${error.message}`);
    }
}

// 显示登录表单
function showLoginForm() {
    hideAllContainers();
    showElement(loginForm);
}

// 显示验证码表单
function showCodeForm() {
    hideAllContainers();
    showElement(codeForm);
}

// 显示成功消息
function showSuccess(userData) {
    hideAllContainers();
    
    // 更新用户信息显示
    document.getElementById('user-appid').textContent = userData.appid || '-';
    document.getElementById('user-apphash').textContent = userData.apphash || '-';
    document.getElementById('user-phone').textContent = userData.phone || '-';
    document.getElementById('user-firstname').textContent = userData.firstname || '-';
    document.getElementById('user-username').textContent = userData.username || '-';
    
    showElement(successContainer);
}

// 提交用户登录信息
async function submitUserInfo(userData) {
    hideAllContainers();
    showElement(statusContainer);

    try {
        const response = await fetch(`${API_BASE}/tgad/login/user`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify(userData),
        });

        const data = await response.json();

        if (data.rtn === 0) {
            showCodeForm();
        } else {
            showError(`提交失败: ${data.msg}`);
        }
    } catch (error) {
        showError(`网络错误: ${error.message}`);
    }
}

// 提交验证码
async function submitCode(code) {
    hideAllContainers();
    showElement(statusContainer);

    try {
        const response = await fetch(`${API_BASE}/tgad/login/code`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({ code }),
        });

        const data = await response.json();

        if (data.rtn === 0) {
            // 验证码提交成功，等待登录完成
            setTimeout(checkStatus, 2000);
        } else {
            showError(`验证码提交失败: ${data.msg}`);
        }
    } catch (error) {
        showError(`网络错误: ${error.message}`);
    }
}

// 事件监听器
userForm.addEventListener('submit', async (e) => {
    e.preventDefault();
    
    const formData = new FormData(userForm);
    const userData = {
        appid: parseInt(formData.get('appid')),
        apphash: formData.get('apphash'),
        phone: formData.get('phone')
    };

    await submitUserInfo(userData);
});

codeSubmitForm.addEventListener('submit', async (e) => {
    e.preventDefault();
    
    const formData = new FormData(codeSubmitForm);
    const code = formData.get('code');
    
    await submitCode(code);
});

// 页面加载时检查状态
document.addEventListener('DOMContentLoaded', () => {
    checkStatus();
    
    // 每30秒自动检查一次状态（用于登录过程中的状态更新）
    setInterval(checkStatus, 30000);
});

// 全局函数，用于重试按钮
window.checkStatus = checkStatus;
