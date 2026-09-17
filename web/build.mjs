// 木鱼前端构建脚本：使用 esbuild 打包 Preact 应用并生成 dist 目录。
import { build, context } from 'esbuild';
import { mkdir, readFile, writeFile, copyFile } from 'node:fs/promises';
import { existsSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const root = path.dirname(fileURLToPath(import.meta.url));
const srcDir = path.join(root, 'src');
const distDir = path.join(root, 'dist');
const watch = process.argv.includes('--watch');

const buildOptions = {
  entryPoints: [path.join(srcDir, 'main.jsx')],
  bundle: true,
  outfile: path.join(distDir, 'app.js'),
  format: 'iife',
  target: ['es2020'],
  jsx: 'automatic',
  jsxImportSource: 'preact',
  minify: !watch,
  sourcemap: watch ? 'inline' : false,
  legalComments: 'none',
  logLevel: 'info',
};

async function copyStatic() {
  await mkdir(distDir, { recursive: true });
  const html = await readFile(path.join(srcDir, 'index.html'), 'utf8');
  await writeFile(path.join(distDir, 'index.html'), html);
  const icon = path.join(srcDir, 'favicon.svg');
  if (existsSync(icon)) {
    await copyFile(icon, path.join(distDir, 'favicon.svg'));
  }
}

async function main() {
  await mkdir(distDir, { recursive: true });
  await copyStatic();
  if (watch) {
    const ctx = await context(buildOptions);
    await ctx.watch();
    console.log('[木鱼] 开发模式已启动，正在监听文件变化…');
  } else {
    await build(buildOptions);
    console.log('[木鱼] 前端构建完成 → web/dist');
  }
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
