<script setup lang="ts">
import { appState } from "@/utils/app-state";
import { actualTheme } from "@/utils/theme";
import { getLocale } from "@/locales";
import {
  darkTheme,
  NConfigProvider,
  NDialogProvider,
  NLoadingBarProvider,
  NMessageProvider,
  useLoadingBar,
  useMessage,
  type GlobalTheme,
  type GlobalThemeOverrides,
  zhCN,
  enUS,
  jaJP,
  dateZhCN,
  dateEnUS,
  dateJaJP,
} from "naive-ui";
import { computed, defineComponent, watch } from "vue";

// 自定义主题配置 - 根据主题动态调整
const themeOverrides = computed<GlobalThemeOverrides>(() => {
  const baseOverrides: GlobalThemeOverrides = {
    common: {
      primaryColor: "#9b5f46",
      primaryColorHover: "#8d523d",
      primaryColorPressed: "#774331",
      primaryColorSuppl: "rgba(193, 95, 60, 0.1)",
      bodyColor: "#fbfaf7",
      cardColor: "#ffffff",
      modalColor: "#ffffff",
      popoverColor: "#ffffff",
      tableColor: "#ffffff",
      inputColor: "#fffefa",
      actionColor: "#f7f5ef",
      textColorBase: "#191919",
      textColor1: "#191919",
      textColor2: "#5f5a52",
      textColor3: "#948d82",
      borderColor: "#e4ded4",
      dividerColor: "rgba(25, 25, 25, 0.07)",
      borderRadius: "8px",
      borderRadiusSmall: "6px",
      fontFamily:
        "'Anthropic Sans Text', 'Tiempos Text', 'Noto Serif SC', 'Microsoft YaHei UI', 'PingFang SC', 'Microsoft YaHei', -apple-system, BlinkMacSystemFont, 'Segoe UI', serif",
    },
    Card: {
      paddingMedium: "24px",
    },
    Button: {
      fontWeight: "600",
      heightMedium: "40px",
      heightLarge: "48px",
      color: "#ffffff",
      colorHover: "#fbfaf7",
      colorPressed: "#f7f5ef",
      colorFocus: "#ffffff",
      border: "1px solid #e4ded4",
      borderHover: "1px solid rgba(193, 95, 60, 0.28)",
      borderPressed: "1px solid rgba(193, 95, 60, 0.36)",
      borderFocus: "1px solid rgba(193, 95, 60, 0.36)",
      textColor: "#5f5a52",
      textColorHover: "#191919",
      textColorPressed: "#191919",
      colorPrimary: "#9b5f46",
      colorHoverPrimary: "#8d523d",
      colorPressedPrimary: "#774331",
      colorFocusPrimary: "#9b5f46",
      borderPrimary: "1px solid #9b5f46",
      borderHoverPrimary: "1px solid #8d523d",
      borderPressedPrimary: "1px solid #774331",
      borderFocusPrimary: "1px solid #9b5f46",
    },
    Input: {
      heightMedium: "40px",
      heightLarge: "48px",
      color: "#fffefa",
      colorFocus: "#ffffff",
      border: "1px solid #e4ded4",
      borderHover: "1px solid rgba(193, 95, 60, 0.28)",
      borderFocus: "1px solid rgba(193, 95, 60, 0.36)",
      boxShadowFocus: "0 0 0 2px rgba(193, 95, 60, 0.08)",
      placeholderColor: "#948d82",
    },
    Menu: {
      itemHeight: "42px",
      itemTextColor: "#5f5a52",
      itemTextColorHover: "#191919",
      itemTextColorActive: "#8d523d",
      itemColorHover: "rgba(193, 95, 60, 0.06)",
      itemColorActive: "rgba(193, 95, 60, 0.08)",
    },
    Tag: {
      borderRadius: "6px",
    },
    LoadingBar: {
      colorLoading: "#9b5f46",
      colorError: "#b74e49",
      height: "3px",
    },
  };

  // 暗黑模式下的特殊覆盖
  if (actualTheme.value === "dark") {
    return {
      ...baseOverrides,
      common: {
        ...baseOverrides.common,
        // 分层对比：浅色外层背景，深黑色内容
        bodyColor: "#24211d", // 外层背景
        cardColor: "#191919", // 卡片内容
        modalColor: "#191919", // 模态框
        popoverColor: "#191919", // 弹出层
        tableColor: "#191919", // 表格
        inputColor: "#24211d", // 输入框
        actionColor: "#24211d", // 操作区域
        textColorBase: "#f4f3ee", // 文字 - 浅色高对比
        textColor1: "#f4f3ee",
        textColor2: "#d2ccc0",
        textColor3: "#b1ada1",
        borderColor: "rgba(244, 243, 238, 0.1)",
        dividerColor: "rgba(244, 243, 238, 0.06)",
      },
      Card: {
        ...baseOverrides.Card,
        color: "#191919", // 卡片背景
        textColor: "#f4f3ee",
        borderColor: "rgba(244, 243, 238, 0.08)",
      },
      Input: {
        ...baseOverrides.Input,
        color: "#24211d", // 输入框背景
        textColor: "#f4f3ee",
        colorFocus: "#24211d",
        borderHover: "rgba(193, 95, 60, 0.4)",
        borderFocus: "rgba(193, 95, 60, 0.58)",
        placeholderColor: "#8f877b",
      },
      Select: {
        peers: {
          InternalSelection: {
            textColor: "#f4f3ee",
            color: "#24211d",
            placeholderColor: "#8f877b",
          },
        },
      },
      DataTable: {
        tdColor: "#191919", // 表格单元格
        thColor: "#24211d", // 表头
        thTextColor: "#f4f3ee",
        tdTextColor: "#f4f3ee",
        borderColor: "rgba(244, 243, 238, 0.08)",
      },
      Tag: {
        textColor: "#f4f3ee",
      },
      Pagination: {
        itemTextColor: "#d2ccc0",
        itemTextColorActive: "#f4f3ee",
        itemColor: "#24211d",
        itemColorActive: "#3a342d",
      },
      DatePicker: {
        itemTextColor: "#f4f3ee",
        itemColorActive: "#24211d",
        panelColor: "#191919",
      },
      Message: {
        color: "#2d2924", // 消息背景
        textColor: "#f4f3ee",
        iconColor: "#f4f3ee",
        borderRadius: "8px",
        colorInfo: "#2d2924",
        colorSuccess: "#2d2924",
        colorWarning: "#2d2924",
        colorError: "#2d2924",
        colorLoading: "#2d2924",
      },
      LoadingBar: {
        ...baseOverrides.LoadingBar,
      },
      Notification: {
        color: "#2d2924", // 通知背景
        textColor: "#f4f3ee",
        titleTextColor: "#f4f3ee",
        descriptionTextColor: "#d2ccc0",
        borderRadius: "8px",
      },
    };
  }

  return baseOverrides;
});

// 根据当前主题动态返回主题对象
const theme = computed<GlobalTheme | undefined>(() => {
  return actualTheme.value === "dark" ? darkTheme : undefined;
});

// 根据当前语言返回对应的 locale 配置
const locale = computed(() => {
  const currentLocale = getLocale();
  switch (currentLocale) {
    case "zh-CN":
      return zhCN;
    case "en-US":
      return enUS;
    case "ja-JP":
      return jaJP;
    default:
      return zhCN;
  }
});

// 根据当前语言返回对应的日期 locale 配置
const dateLocale = computed(() => {
  const currentLocale = getLocale();
  switch (currentLocale) {
    case "zh-CN":
      return dateZhCN;
    case "en-US":
      return dateEnUS;
    case "ja-JP":
      return dateJaJP;
    default:
      return dateZhCN;
  }
});

function useGlobalMessage() {
  window.$message = useMessage();
}

const LoadingBar = defineComponent({
  setup() {
    const loadingBar = useLoadingBar();
    watch(
      () => appState.loading,
      loading => {
        if (loading) {
          loadingBar.start();
        } else {
          loadingBar.finish();
        }
      }
    );
    return () => null;
  },
});

const Message = defineComponent({
  setup() {
    useGlobalMessage();
    return () => null;
  },
});
</script>

<template>
  <n-config-provider
    :theme="theme"
    :theme-overrides="themeOverrides"
    :locale="locale"
    :date-locale="dateLocale"
  >
    <n-loading-bar-provider>
      <n-message-provider placement="top-right">
        <n-dialog-provider>
          <slot />
          <loading-bar />
          <message />
        </n-dialog-provider>
      </n-message-provider>
    </n-loading-bar-provider>
  </n-config-provider>
</template>
