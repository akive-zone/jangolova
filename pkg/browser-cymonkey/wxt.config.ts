import { defineConfig } from 'wxt';

export default defineConfig({
  outDirTemplate: '{{browser}}-mv{{manifestVersion}}{{modeSuffix}}',
  manifest: ({ mode }) => {
    const spoke = mode === 'spoke';
    return {
          name: spoke ? 'Xallet Spoke: Jangolova Browser Extension' : 'Jangolova Browser Extension',
          description: spoke
        ? 'Jangolova Browser Extension for Xallet Hub and standalone operation'
        : 'Jangolova Browser Extension with packaged scripts, overlays, storage, and request rules',
      permissions: [
        'declarativeNetRequest',
        'declarativeNetRequestWithHostAccess',
        'scripting',
        'storage',
        'tabs',
        ...(spoke ? ['management'] : []),
      ],
      host_permissions: ['<all_urls>'],
      externally_connectable: spoke ? { ids: ['*'] } : undefined,
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
    };
  },
});
