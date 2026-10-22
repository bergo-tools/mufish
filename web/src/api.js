// 统一的 API 客户端。
// 所有接口返回 { code, msg, data }，成功时直接返回 data，失败时抛出 Error。

const BASE = '/api/v1';

async function request(method, path, { body, params, raw } = {}) {
  let url = BASE + path;
  if (params) {
    const search = new URLSearchParams();
    Object.entries(params).forEach(([k, v]) => {
      if (v === undefined || v === null || v === '') return;
      search.append(k, String(v));
    });
    const qs = search.toString();
    if (qs) url += (url.includes('?') ? '&' : '?') + qs;
  }

  const init = {
    method,
    credentials: 'same-origin',
    headers: { 'X-Requested-With': 'fetch' },
  };
  if (body !== undefined) {
    init.headers['Content-Type'] = 'application/json';
    init.body = JSON.stringify(body);
  }

  let resp;
  try {
    resp = await fetch(url, init);
  } catch (e) {
    throw new Error('网络连接失败，请确认服务仍在运行');
  }

  // 登录接口返回 401 表示“用户名或口令错误”，不是会话失效；
  // 不能触发全局退出事件，也不能覆盖服务端给出的具体提示。
  if (resp.status === 401 && path !== '/auth/login') {
    if (typeof window !== 'undefined') {
      window.dispatchEvent(new Event('mufish:unauthorized'));
    }
    const err = new Error('登录状态已失效，请重新登录');
    err.unauthorized = true;
    throw err;
  }
  if (raw) {
    if (!resp.ok) throw new Error('请求失败（' + resp.status + '）');
    return resp;
  }

  const text = await resp.text();
  let payload;
  try {
    payload = text ? JSON.parse(text) : {};
  } catch (e) {
    throw new Error('服务器返回了无法解析的内容');
  }
  if (!resp.ok || (payload.code !== 0 && payload.code !== undefined)) {
    const err = new Error(payload.msg || '请求失败（' + resp.status + '）');
    err.status = resp.status;
    throw err;
  }
  return payload.data;
}


// 从 Content-Disposition 中解析服务端文件名，优先使用 filename*。
export function filenameFromDisposition(disposition, fallback = '导出.csv') {
  if (!disposition) return fallback;
  const star = disposition.match(/filename\*\s*=\s*UTF-8''([^;]+)/i);
  if (star) {
    try {
      return decodeURIComponent(star[1].trim().replace(/^"|"$/g, ''));
    } catch (e) {
      /* 编码异常时使用普通 filename 或 fallback */
    }
  }
  const plain = disposition.match(/filename\s*=\s*"?([^";]+)"?/i);
  if (plain) return plain[1].trim();
  return fallback;
}

export const api = {
  get: (path, params) => request('GET', path, { params }),
  post: (path, body) => request('POST', path, { body: body === undefined ? {} : body }),
  put: (path, body) => request('PUT', path, { body }),
  del: (path) => request('DELETE', path),
  async download(path, params) {
    let url = BASE + path;
    if (params) {
      const search = new URLSearchParams();
      Object.entries(params).forEach(([k, v]) => {
        if (v === undefined || v === null || v === '') return;
        search.append(k, String(v));
      });
      const qs = search.toString();
      if (qs) url += '?' + qs;
    }

    let resp;
    try {
      resp = await fetch(url, {
        credentials: 'same-origin',
        headers: { 'X-Requested-With': 'fetch' },
      });
    } catch (e) {
      throw new Error('网络连接失败，请确认服务仍在运行');
    }
    if (resp.status === 401) {
      if (typeof window !== 'undefined') {
        window.dispatchEvent(new Event('mufish:unauthorized'));
      }
      throw new Error('登录状态已失效，请重新登录');
    }
    if (!resp.ok) {
      let message = '导出失败（' + resp.status + '）';
      try {
        const payload = await resp.json();
        if (payload && payload.msg) message = payload.msg;
      } catch (e) {
        /* 非 JSON 错误响应，保留状态码提示 */
      }
      throw new Error(message);
    }

    const blob = await resp.blob();
    const filename = filenameFromDisposition(
      resp.headers.get('Content-Disposition') || '',
      '导出.csv',
    );
    const objectUrl = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = objectUrl;
    a.download = filename;
    a.rel = 'noopener';
    document.body.appendChild(a);
    a.click();
    a.remove();
    setTimeout(() => URL.revokeObjectURL(objectUrl), 1000);
  },
};

// ---------------------------------------------------------------------------
// 业务接口封装
// ---------------------------------------------------------------------------

export const Api = {
  meta: () => api.get('/meta'),
  login: (username, password) => api.post('/auth/login', { username, password }),
  logout: () => api.post('/auth/logout'),
  me: () => api.get('/auth/me'),
  changePassword: (old_password, new_password) =>
    api.post('/auth/password', { old_password, new_password }),

  dashboard: () => api.get('/dashboard'),

  categories: () => api.get('/categories'),
  categoriesFlat: () => api.get('/categories/flat'),
  saveCategory: (data, id) => (id ? api.put(`/categories/${id}`, data) : api.post('/categories', data)),
  deleteCategory: (id) => api.del(`/categories/${id}`),

  products: (params) => api.get('/products', params),
  productOptions: (keyword, warehouse_id) => api.get('/products/options', { keyword, warehouse_id }),
  product: (id) => api.get(`/products/${id}`),
  saveProduct: (data, id) => (id ? api.put(`/products/${id}`, data) : api.post('/products', data)),
  deleteProduct: (id) => api.del(`/products/${id}`),
  importProducts: (text) => api.post('/products/import', { text }),

  partners: (params) => api.get('/partners', params),
  partnerOptions: (type, keyword) => api.get('/partners/options', { type, keyword }),
  partner: (id) => api.get(`/partners/${id}`),
  savePartner: (data, id) => (id ? api.put(`/partners/${id}`, data) : api.post('/partners', data)),
  deletePartner: (id) => api.del(`/partners/${id}`),

  warehouses: () => api.get('/warehouses'),
  warehouseOptions: () => api.get('/warehouses/options'),
  saveWarehouse: (data, id) => (id ? api.put(`/warehouses/${id}`, data) : api.post('/warehouses', data)),
  deleteWarehouse: (id) => api.del(`/warehouses/${id}`),

  accounts: () => api.get('/accounts'),
  accountOptions: () => api.get('/accounts/options'),
  saveAccount: (data, id) => (id ? api.put(`/accounts/${id}`, data) : api.post('/accounts', data)),
  adjustAccount: (id, balance, reason) =>
    api.post(`/accounts/${id}/adjust`, { balance, reason }),
  accountAdjustments: (id, params) => api.get(`/accounts/${id}/adjustments`, params),
  deleteAccount: (id) => api.del(`/accounts/${id}`),

  purchases: (params) => api.get('/purchases', params),
  purchase: (id) => api.get(`/purchases/${id}`),
  savePurchase: (data, id) => (id ? api.put(`/purchases/${id}`, data) : api.post('/purchases', data)),
  confirmPurchase: (id) => api.post(`/purchases/${id}/confirm`),
  voidPurchase: (id) => api.post(`/purchases/${id}/void`),
  deletePurchase: (id) => api.del(`/purchases/${id}`),

  sales: (params) => api.get('/sales', params),
  sale: (id) => api.get(`/sales/${id}`),
  saveSales: (data, id) => (id ? api.put(`/sales/${id}`, data) : api.post('/sales', data)),
  confirmSales: (id) => api.post(`/sales/${id}/confirm`),
  voidSales: (id) => api.post(`/sales/${id}/void`),
  deleteSales: (id) => api.del(`/sales/${id}`),

  stocks: (params) => api.get('/stocks', params),
  stockSummary: (params) => api.get('/stocks/summary', params),
  stockAlerts: (params) => api.get('/stocks/alerts', params),
  ledger: (params) => api.get('/stocks/ledger', params),

  stockTakes: (params) => api.get('/stock-takes', params),
  stockTake: (id) => api.get(`/stock-takes/${id}`),
  saveStockTake: (data, id) =>
    id ? api.put(`/stock-takes/${id}`, data) : api.post('/stock-takes', data),
  finishStockTake: (id) => api.post(`/stock-takes/${id}/finish`),
  voidStockTake: (id) => api.post(`/stock-takes/${id}/void`),
  deleteStockTake: (id) => api.del(`/stock-takes/${id}`),

  transfers: (params) => api.get('/transfers', params),
  transfer: (id) => api.get(`/transfers/${id}`),
  saveTransfer: (data, id) => (id ? api.put(`/transfers/${id}`, data) : api.post('/transfers', data)),
  confirmTransfer: (id) => api.post(`/transfers/${id}/confirm`),
  voidTransfer: (id) => api.post(`/transfers/${id}/void`),
  deleteTransfer: (id) => api.del(`/transfers/${id}`),

  payments: (params) => api.get('/payments', params),
  payment: (id) => api.get(`/payments/${id}`),
  savePayment: (data, id) => (id ? api.put(`/payments/${id}`, data) : api.post('/payments', data)),
  confirmPayment: (id) => api.post(`/payments/${id}/confirm`),
  voidPayment: (id) => api.post(`/payments/${id}/void`),
  deletePayment: (id) => api.del(`/payments/${id}`),
  unpaidOrders: (type, partner_id) => api.get('/payments/unpaid-orders', { type, partner_id }),

  reportInout: (params) => api.get('/reports/inout', params),
  reportSalesRank: (params) => api.get('/reports/sales-rank', params),
  reportPurchaseRank: (params) => api.get('/reports/purchase-rank', params),
  reportCustomerRank: (params) => api.get('/reports/customer-rank', params),
  reportSupplierRank: (params) => api.get('/reports/supplier-rank', params),
  reportProfit: (params) => api.get('/reports/profit', params),
  reportArAp: (params) => api.get('/reports/ar-ap', params),
  reportPurchaseDetail: (params) => api.get('/reports/purchase-detail', params),
  reportSalesDetail: (params) => api.get('/reports/sales-detail', params),
  reportCashFlow: (params) => api.get('/reports/cash-flow', params),

  users: (params) => api.get('/users', params),
  createUser: (data) => api.post('/users', data),
  updateUser: (id, data) => api.put(`/users/${id}`, data),
  deleteUser: (id) => api.del(`/users/${id}`),
  resetPassword: (id, password) => api.post(`/users/${id}/reset-password`, { password }),

  roles: () => api.get('/roles'),
  permissions: () => api.get('/permissions'),
  createRole: (data) => api.post('/roles', data),
  updateRole: (id, data) => api.put(`/roles/${id}`, data),
  deleteRole: (id) => api.del(`/roles/${id}`),

  logs: (params) => api.get('/logs', params),
  logModules: () => api.get('/logs/modules'),

  settings: () => api.get('/settings'),
  saveSettings: (data) => api.put('/settings', data),

  systemInfo: () => api.get('/system/info'),
  backup: () => api.post('/system/backup'),
  backups: () => api.get('/system/backups'),
  restore: (name) => api.post('/system/restore', { name }),

  exportProducts: (params) => api.download('/products/export', params),
  exportLedger: (params) => api.download('/stocks/ledger/export', params),
  exportInout: (params) => api.download('/reports/inout/export', params),
  exportArAp: (params) => api.download('/reports/ar-ap/export', params),
};
