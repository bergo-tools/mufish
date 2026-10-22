-- 木鱼进销存 数据库结构
-- 所有金额字段以「分」为单位（INTEGER），单价字段以「万分之一元」为单位（INTEGER），
-- 数量为 REAL（最多 4 位小数）。

-- ===========================================================================
-- 系统
-- ===========================================================================

CREATE TABLE IF NOT EXISTS roles (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    code        TEXT    NOT NULL UNIQUE,
    name        TEXT    NOT NULL,
    remark      TEXT    NOT NULL DEFAULT '',
    is_system   INTEGER NOT NULL DEFAULT 0,
    created_at  TEXT    NOT NULL DEFAULT '',
    updated_at  TEXT    NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS role_permissions (
    role_id  INTEGER NOT NULL,
    perm     TEXT    NOT NULL,
    PRIMARY KEY (role_id, perm)
);

CREATE TABLE IF NOT EXISTS users (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    username      TEXT    NOT NULL UNIQUE,
    password      TEXT    NOT NULL,
    name          TEXT    NOT NULL DEFAULT '',
    phone         TEXT    NOT NULL DEFAULT '',
    email         TEXT    NOT NULL DEFAULT '',
    role_id       INTEGER NOT NULL DEFAULT 0,
    status        INTEGER NOT NULL DEFAULT 1,
    is_super      INTEGER NOT NULL DEFAULT 0,
    remark        TEXT    NOT NULL DEFAULT '',
    last_login_at TEXT    NOT NULL DEFAULT '',
    created_at    TEXT    NOT NULL DEFAULT '',
    updated_at    TEXT    NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_users_role ON users(role_id);

CREATE TABLE IF NOT EXISTS sessions (
    token       TEXT    PRIMARY KEY,
    user_id     INTEGER NOT NULL,
    ip          TEXT    NOT NULL DEFAULT '',
    user_agent  TEXT    NOT NULL DEFAULT '',
    created_at  TEXT    NOT NULL DEFAULT '',
    expires_at  TEXT    NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions(user_id);

CREATE TABLE IF NOT EXISTS audit_logs (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id    INTEGER NOT NULL DEFAULT 0,
    username   TEXT    NOT NULL DEFAULT '',
    module     TEXT    NOT NULL DEFAULT '',
    action     TEXT    NOT NULL DEFAULT '',
    detail     TEXT    NOT NULL DEFAULT '',
    ip         TEXT    NOT NULL DEFAULT '',
    created_at TEXT    NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_audit_created ON audit_logs(created_at);

CREATE TABLE IF NOT EXISTS settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL DEFAULT ''
);

-- 单据流水号
CREATE TABLE IF NOT EXISTS doc_seqs (
    kind TEXT    NOT NULL,
    day  TEXT    NOT NULL,
    seq  INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (kind, day)
);

-- ===========================================================================
-- 基础资料
-- ===========================================================================

CREATE TABLE IF NOT EXISTS categories (
    id        INTEGER PRIMARY KEY AUTOINCREMENT,
    name      TEXT    NOT NULL,
    parent_id INTEGER NOT NULL DEFAULT 0,
    sort      INTEGER NOT NULL DEFAULT 0,
    remark    TEXT    NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_categories_parent ON categories(parent_id);

CREATE TABLE IF NOT EXISTS products (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    code             TEXT    NOT NULL UNIQUE,
    name             TEXT    NOT NULL,
    spec             TEXT    NOT NULL DEFAULT '',
    unit             TEXT    NOT NULL DEFAULT '个',
    category_id      INTEGER NOT NULL DEFAULT 0,
    barcode          TEXT    NOT NULL DEFAULT '',
    brand            TEXT    NOT NULL DEFAULT '',
    purchase_price   INTEGER NOT NULL DEFAULT 0,
    sale_price       INTEGER NOT NULL DEFAULT 0,
    safety_stock     REAL    NOT NULL DEFAULT 0,
    safety_stock_max REAL    NOT NULL DEFAULT 0,
    status           INTEGER NOT NULL DEFAULT 1,
    remark           TEXT    NOT NULL DEFAULT '',
    created_at       TEXT    NOT NULL DEFAULT '',
    updated_at       TEXT    NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_products_name ON products(name);
CREATE INDEX IF NOT EXISTS idx_products_category ON products(category_id);

CREATE TABLE IF NOT EXISTS partners (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    code            TEXT    NOT NULL UNIQUE,
    name            TEXT    NOT NULL,
    type            TEXT    NOT NULL DEFAULT 'customer',
    contact         TEXT    NOT NULL DEFAULT '',
    phone           TEXT    NOT NULL DEFAULT '',
    address         TEXT    NOT NULL DEFAULT '',
    tax_no          TEXT    NOT NULL DEFAULT '',
    bank_name       TEXT    NOT NULL DEFAULT '',
    bank_account    TEXT    NOT NULL DEFAULT '',
    credit_limit    INTEGER NOT NULL DEFAULT 0,
    init_receivable INTEGER NOT NULL DEFAULT 0,
    init_payable    INTEGER NOT NULL DEFAULT 0,
    status          INTEGER NOT NULL DEFAULT 1,
    remark          TEXT    NOT NULL DEFAULT '',
    created_at      TEXT    NOT NULL DEFAULT '',
    updated_at      TEXT    NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_partners_name ON partners(name);
CREATE INDEX IF NOT EXISTS idx_partners_type ON partners(type);

CREATE TABLE IF NOT EXISTS warehouses (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    code       TEXT    NOT NULL UNIQUE,
    name       TEXT    NOT NULL,
    address    TEXT    NOT NULL DEFAULT '',
    contact    TEXT    NOT NULL DEFAULT '',
    phone      TEXT    NOT NULL DEFAULT '',
    is_default INTEGER NOT NULL DEFAULT 0,
    sort       INTEGER NOT NULL DEFAULT 0,
    status     INTEGER NOT NULL DEFAULT 1,
    remark     TEXT    NOT NULL DEFAULT '',
    created_at TEXT    NOT NULL DEFAULT '',
    updated_at TEXT    NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS accounts (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    code       TEXT    NOT NULL UNIQUE,
    name       TEXT    NOT NULL,
    type       TEXT    NOT NULL DEFAULT 'cash',
    balance    INTEGER NOT NULL DEFAULT 0,
    is_default INTEGER NOT NULL DEFAULT 0,
    sort       INTEGER NOT NULL DEFAULT 0,
    status     INTEGER NOT NULL DEFAULT 1,
    remark     TEXT    NOT NULL DEFAULT '',
    created_at TEXT    NOT NULL DEFAULT '',
    updated_at TEXT    NOT NULL DEFAULT ''
);

-- ===========================================================================
-- 采购
-- ===========================================================================

CREATE TABLE IF NOT EXISTS purchase_orders (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    order_no        TEXT    NOT NULL UNIQUE,
    supplier_id     INTEGER NOT NULL DEFAULT 0,
    supplier_name   TEXT    NOT NULL DEFAULT '',
    warehouse_id    INTEGER NOT NULL DEFAULT 0,
    warehouse_name  TEXT    NOT NULL DEFAULT '',
    order_date      TEXT    NOT NULL DEFAULT '',
    status          INTEGER NOT NULL DEFAULT 0,
    total_qty       REAL    NOT NULL DEFAULT 0,
    total_amount    INTEGER NOT NULL DEFAULT 0,
    discount_amount INTEGER NOT NULL DEFAULT 0,
    other_fee       INTEGER NOT NULL DEFAULT 0,
    settle_amount   INTEGER NOT NULL DEFAULT 0,
    paid_amount     INTEGER NOT NULL DEFAULT 0,
    remark          TEXT    NOT NULL DEFAULT '',
    created_by      INTEGER NOT NULL DEFAULT 0,
    created_by_name TEXT    NOT NULL DEFAULT '',
    confirmed_at    TEXT    NOT NULL DEFAULT '',
    confirmed_by    INTEGER NOT NULL DEFAULT 0,
    created_at      TEXT    NOT NULL DEFAULT '',
    updated_at      TEXT    NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_purchase_date ON purchase_orders(order_date);
CREATE INDEX IF NOT EXISTS idx_purchase_supplier ON purchase_orders(supplier_id);
CREATE INDEX IF NOT EXISTS idx_purchase_status ON purchase_orders(status);

CREATE TABLE IF NOT EXISTS purchase_items (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    order_id     INTEGER NOT NULL,
    product_id   INTEGER NOT NULL,
    product_code TEXT    NOT NULL DEFAULT '',
    product_name TEXT    NOT NULL DEFAULT '',
    spec         TEXT    NOT NULL DEFAULT '',
    unit         TEXT    NOT NULL DEFAULT '',
    qty          REAL    NOT NULL DEFAULT 0,
    price        INTEGER NOT NULL DEFAULT 0,
    amount       INTEGER NOT NULL DEFAULT 0,
    remark       TEXT    NOT NULL DEFAULT '',
    sort         INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_purchase_items_order ON purchase_items(order_id);
CREATE INDEX IF NOT EXISTS idx_purchase_items_product ON purchase_items(product_id);

-- ===========================================================================
-- 销售
-- ===========================================================================

CREATE TABLE IF NOT EXISTS sales_orders (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    order_no        TEXT    NOT NULL UNIQUE,
    customer_id     INTEGER NOT NULL DEFAULT 0,
    customer_name   TEXT    NOT NULL DEFAULT '',
    warehouse_id    INTEGER NOT NULL DEFAULT 0,
    warehouse_name  TEXT    NOT NULL DEFAULT '',
    order_date      TEXT    NOT NULL DEFAULT '',
    status          INTEGER NOT NULL DEFAULT 0,
    total_qty       REAL    NOT NULL DEFAULT 0,
    total_amount    INTEGER NOT NULL DEFAULT 0,
    cost_amount     INTEGER NOT NULL DEFAULT 0,
    discount_amount INTEGER NOT NULL DEFAULT 0,
    other_fee       INTEGER NOT NULL DEFAULT 0,
    settle_amount   INTEGER NOT NULL DEFAULT 0,
    received_amount INTEGER NOT NULL DEFAULT 0,
    remark          TEXT    NOT NULL DEFAULT '',
    created_by      INTEGER NOT NULL DEFAULT 0,
    created_by_name TEXT    NOT NULL DEFAULT '',
    confirmed_at    TEXT    NOT NULL DEFAULT '',
    confirmed_by    INTEGER NOT NULL DEFAULT 0,
    created_at      TEXT    NOT NULL DEFAULT '',
    updated_at      TEXT    NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_sales_date ON sales_orders(order_date);
CREATE INDEX IF NOT EXISTS idx_sales_customer ON sales_orders(customer_id);
CREATE INDEX IF NOT EXISTS idx_sales_status ON sales_orders(status);

CREATE TABLE IF NOT EXISTS sales_items (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    order_id     INTEGER NOT NULL,
    product_id   INTEGER NOT NULL,
    product_code TEXT    NOT NULL DEFAULT '',
    product_name TEXT    NOT NULL DEFAULT '',
    spec         TEXT    NOT NULL DEFAULT '',
    unit         TEXT    NOT NULL DEFAULT '',
    qty          REAL    NOT NULL DEFAULT 0,
    price        INTEGER NOT NULL DEFAULT 0,
    amount       INTEGER NOT NULL DEFAULT 0,
    cost_price   INTEGER NOT NULL DEFAULT 0,
    remark       TEXT    NOT NULL DEFAULT '',
    sort         INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_sales_items_order ON sales_items(order_id);
CREATE INDEX IF NOT EXISTS idx_sales_items_product ON sales_items(product_id);

-- ===========================================================================
-- 库存
-- ===========================================================================

CREATE TABLE IF NOT EXISTS stocks (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    warehouse_id INTEGER NOT NULL,
    product_id   INTEGER NOT NULL,
    qty          REAL    NOT NULL DEFAULT 0,
    cost_price   INTEGER NOT NULL DEFAULT 0,
    amount       INTEGER NOT NULL DEFAULT 0,
    updated_at   TEXT    NOT NULL DEFAULT '',
    UNIQUE (warehouse_id, product_id)
);
CREATE INDEX IF NOT EXISTS idx_stocks_product ON stocks(product_id);

CREATE TABLE IF NOT EXISTS stock_moves (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    warehouse_id  INTEGER NOT NULL,
    product_id    INTEGER NOT NULL,
    biz_type      TEXT    NOT NULL DEFAULT '',
    biz_id        INTEGER NOT NULL DEFAULT 0,
    biz_no        TEXT    NOT NULL DEFAULT '',
    biz_date      TEXT    NOT NULL DEFAULT '',
    direction     INTEGER NOT NULL DEFAULT 1,
    qty           REAL    NOT NULL DEFAULT 0,
    price         INTEGER NOT NULL DEFAULT 0,
    amount        INTEGER NOT NULL DEFAULT 0,
    balance_after REAL    NOT NULL DEFAULT 0,
    remark        TEXT    NOT NULL DEFAULT '',
    created_by    INTEGER NOT NULL DEFAULT 0,
    created_by_name TEXT  NOT NULL DEFAULT '',
    created_at    TEXT    NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_moves_wh_product ON stock_moves(warehouse_id, product_id, id);
CREATE INDEX IF NOT EXISTS idx_moves_created ON stock_moves(created_at);
CREATE INDEX IF NOT EXISTS idx_moves_biz_date ON stock_moves(biz_date);
CREATE INDEX IF NOT EXISTS idx_moves_biz ON stock_moves(biz_type, biz_id);

CREATE TABLE IF NOT EXISTS stock_takes (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    take_no         TEXT    NOT NULL UNIQUE,
    warehouse_id    INTEGER NOT NULL DEFAULT 0,
    warehouse_name  TEXT    NOT NULL DEFAULT '',
    take_date       TEXT    NOT NULL DEFAULT '',
    status          INTEGER NOT NULL DEFAULT 0,
    diff_qty        REAL    NOT NULL DEFAULT 0,
    diff_amount     INTEGER NOT NULL DEFAULT 0,
    remark          TEXT    NOT NULL DEFAULT '',
    created_by      INTEGER NOT NULL DEFAULT 0,
    created_by_name TEXT    NOT NULL DEFAULT '',
    finished_at     TEXT    NOT NULL DEFAULT '',
    created_at      TEXT    NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_takes_date ON stock_takes(take_date);

CREATE TABLE IF NOT EXISTS stock_take_items (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    take_id      INTEGER NOT NULL,
    product_id   INTEGER NOT NULL,
    product_code TEXT    NOT NULL DEFAULT '',
    product_name TEXT    NOT NULL DEFAULT '',
    spec         TEXT    NOT NULL DEFAULT '',
    unit         TEXT    NOT NULL DEFAULT '',
    book_qty     REAL    NOT NULL DEFAULT 0,
    real_qty     REAL    NOT NULL DEFAULT 0,
    diff_qty     REAL    NOT NULL DEFAULT 0,
    price        INTEGER NOT NULL DEFAULT 0,
    diff_amount  INTEGER NOT NULL DEFAULT 0,
    remark       TEXT    NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_take_items_take ON stock_take_items(take_id);

CREATE TABLE IF NOT EXISTS transfers (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    transfer_no       TEXT    NOT NULL UNIQUE,
    from_warehouse_id INTEGER NOT NULL DEFAULT 0,
    from_name         TEXT    NOT NULL DEFAULT '',
    to_warehouse_id   INTEGER NOT NULL DEFAULT 0,
    to_name           TEXT    NOT NULL DEFAULT '',
    transfer_date     TEXT    NOT NULL DEFAULT '',
    status            INTEGER NOT NULL DEFAULT 0,
    total_qty         REAL    NOT NULL DEFAULT 0,
    total_amount      INTEGER NOT NULL DEFAULT 0,
    remark            TEXT    NOT NULL DEFAULT '',
    created_by        INTEGER NOT NULL DEFAULT 0,
    created_by_name   TEXT    NOT NULL DEFAULT '',
    confirmed_at      TEXT    NOT NULL DEFAULT '',
    created_at        TEXT    NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_transfers_date ON transfers(transfer_date);

CREATE TABLE IF NOT EXISTS transfer_items (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    transfer_id  INTEGER NOT NULL,
    product_id   INTEGER NOT NULL,
    product_code TEXT    NOT NULL DEFAULT '',
    product_name TEXT    NOT NULL DEFAULT '',
    spec         TEXT    NOT NULL DEFAULT '',
    unit         TEXT    NOT NULL DEFAULT '',
    qty          REAL    NOT NULL DEFAULT 0,
    price        INTEGER NOT NULL DEFAULT 0,
    amount       INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_transfer_items_transfer ON transfer_items(transfer_id);

-- ===========================================================================
-- 收付款
-- ===========================================================================

CREATE TABLE IF NOT EXISTS payments (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    payment_no      TEXT    NOT NULL UNIQUE,
    type            TEXT    NOT NULL DEFAULT 'receipt',
    partner_id      INTEGER NOT NULL DEFAULT 0,
    partner_name    TEXT    NOT NULL DEFAULT '',
    account_id      INTEGER NOT NULL DEFAULT 0,
    account_name    TEXT    NOT NULL DEFAULT '',
    pay_date        TEXT    NOT NULL DEFAULT '',
    amount          INTEGER NOT NULL DEFAULT 0,
    method          TEXT    NOT NULL DEFAULT 'bank',
    order_type      TEXT    NOT NULL DEFAULT '',
    order_id        INTEGER NOT NULL DEFAULT 0,
    order_no        TEXT    NOT NULL DEFAULT '',
    status          INTEGER NOT NULL DEFAULT 0,
    remark          TEXT    NOT NULL DEFAULT '',
    created_by      INTEGER NOT NULL DEFAULT 0,
    created_by_name TEXT    NOT NULL DEFAULT '',
    confirmed_at    TEXT    NOT NULL DEFAULT '',
    created_at      TEXT    NOT NULL DEFAULT '',
    updated_at      TEXT    NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_payments_date ON payments(pay_date);
CREATE INDEX IF NOT EXISTS idx_payments_partner ON payments(partner_id);
CREATE INDEX IF NOT EXISTS idx_payments_order ON payments(order_type, order_id);
