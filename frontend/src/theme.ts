import type { GlobalThemeOverrides } from 'naive-ui'

export const themeOverrides: GlobalThemeOverrides = {
  common: {
    primaryColor: '#1669ff',
    primaryColorHover: '#3b82f6',
    primaryColorPressed: '#0052e0',
    primaryColorSuppl: '#1669ff',
    successColor: '#10b981',
    successColorHover: '#34d399',
    successColorPressed: '#059669',
    warningColor: '#f59e0b',
    warningColorHover: '#fbbf24',
    warningColorPressed: '#d97706',
    errorColor: '#ef4444',
    errorColorHover: '#f87171',
    errorColorPressed: '#dc2626',
    infoColor: '#1669ff',
    borderRadius: '12px',
    borderRadiusSmall: '8px',
    fontFamily:
      '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "PingFang SC", "Hiragino Sans GB", "Microsoft YaHei", sans-serif',
  },
  Card: {
    borderRadius: '16px',
    borderColor: '#f1f5f9',
    boxShadow: '0 1px 3px 0 rgba(0, 0, 0, 0.04), 0 1px 2px -1px rgba(0, 0, 0, 0.04)',
    color: '#ffffff',
  },
  Button: {
    borderRadiusMedium: '10px',
    borderRadiusSmall: '8px',
    fontWeight: '500',
  },
  Input: {
    borderRadius: '10px',
    color: '#f8fafc',
    colorFocus: '#ffffff',
    border: '1px solid #e2e8f0',
  },
  Tag: {
    borderRadius: '9999px',
  },
  Layout: {
    color: '#f5f7fa',
    siderColor: '#ffffff',
    headerColor: '#ffffff',
    footerColor: '#ffffff',
  },
  Menu: {
    borderRadius: '10px',
    itemColorHover: '#f1f5f9',
    itemColorActive: '#eef2ff',
    itemTextColorActive: '#1669ff',
    itemIconColorActive: '#1669ff',
    itemTextColorHover: '#1e293b',
    itemIconColorHover: '#1669ff',
  },
}
