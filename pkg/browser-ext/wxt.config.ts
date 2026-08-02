import { defineConfig } from 'wxt';

export default defineConfig({
  outDirTemplate: '{{browser}}-mv{{manifestVersion}}',
  manifest: {
    name: 'Jangolova Browser Extension',
    description: 'Jangolova browser runtime with Cymonkey, Pacman, and runtime-activated Xallet Spook integration',
    permissions: [
      'declarativeNetRequest',
      'declarativeNetRequestWithHostAccess',
      'scripting',
      'storage',
      'tabs',
      'management',
    ],
    host_permissions: ['<all_urls>'],
    externally_connectable: { ids: ['*'] },
    web_accessible_resources: [
      {
        resources: ['cymonkey-main.js', 'augmentations/*'],
        matches: ['<all_urls>'],
      },
    ],
    browser_specific_settings: {
      gecko: {
        id: 'browser-jangolova@jangolova.dev',
        strict_min_version: '128.0',
        data_collection_permissions: {
          required: ['none'],
        },
      },
    },
  },
});
