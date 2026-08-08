import * as vue0 from "vue";
import { InjectionKey, MaybeRefOrGetter, PropType, SlotsType } from "vue";
import * as echarts_core0 from "echarts/core";
import { SetOptionOpts, init } from "echarts/core";
import { TooltipComponentFormatterCallbackParams } from "echarts";

//#region src/types.d.ts
type Injection<T> = MaybeRefOrGetter<T | null>;
type InitType = typeof init;
type InitParameters = Parameters<InitType>;
type Theme = NonNullable<InitParameters[1]>;
type ThemeInjection = Injection<Theme>;
type InitOptions = NonNullable<InitParameters[2]>;
type InitOptionsInjection = Injection<InitOptions>;
type UpdateOptions = SetOptionOpts;
type UpdateOptionsInjection = Injection<UpdateOptions>;
type EChartsType = ReturnType<InitType>;
type SetOptionType = EChartsType["setOption"];
type Option = Parameters<SetOptionType>[0];
type AutoResize = boolean | {
  throttle?: number;
  onResize?: () => void;
};
type LoadingOptions = {
  text?: string;
  textColor?: string;
  fontSize?: number | string;
  fontWeight?: number | string;
  fontStyle?: string;
  fontFamily?: string;
  maskColor?: string;
  showSpinner?: boolean;
  color?: string;
  spinnerRadius?: number;
  lineWidth?: number;
  zlevel?: number;
};
type LoadingOptionsInjection = Injection<LoadingOptions>;
//#endregion
//#region src/composables/api.d.ts
declare const METHOD_NAMES: readonly ["getWidth", "getHeight", "getDom", "getOption", "resize", "dispatchAction", "convertToPixel", "convertFromPixel", "containPixel", "getDataURL", "getConnectedDataURL", "appendData", "clear", "isDisposed", "dispose"];
type MethodName = (typeof METHOD_NAMES)[number];
type PublicMethods = Pick<EChartsType, MethodName>;
//#endregion
//#region src/composables/loading.d.ts
declare const LOADING_OPTIONS_KEY: InjectionKey<LoadingOptionsInjection>;
//#endregion
//#region src/composables/slot.d.ts
type SlotsTypes = SlotsType<Record<"tooltip" | `tooltip-${string}`, TooltipComponentFormatterCallbackParams> & Record<"dataView" | `dataView-${string}`, Option>>;
//#endregion
//#region src/wc.d.ts
interface EChartsElement extends HTMLElement {
  __dispose: (() => void) | null;
}
//#endregion
//#region src/ECharts.d.ts
declare const THEME_KEY: InjectionKey<ThemeInjection>;
declare const INIT_OPTIONS_KEY: InjectionKey<InitOptionsInjection>;
declare const UPDATE_OPTIONS_KEY: InjectionKey<UpdateOptionsInjection>;
declare const _default: vue0.DefineComponent<vue0.ExtractPropTypes<{
  loading: BooleanConstructor;
  loadingOptions: PropType<LoadingOptions>;
  autoresize: PropType<AutoResize>;
  option: PropType<Option>;
  theme: {
    type: PropType<Theme>;
  };
  initOptions: PropType<InitOptions>;
  updateOptions: PropType<UpdateOptions>;
  group: StringConstructor;
  manualUpdate: BooleanConstructor;
}>, {
  setOption: {
    <Opt extends echarts_core0.EChartsCoreOption>(option: Opt, notMerge?: boolean, lazyUpdate?: boolean): void;
    <Opt extends echarts_core0.EChartsCoreOption>(option: Opt, opts?: echarts_core0.SetOptionOpts): void;
  };
  root: vue0.ShallowRef<EChartsElement | undefined, EChartsElement | undefined>;
  chart: vue0.ShallowRef<echarts_core0.ECharts | undefined, echarts_core0.ECharts | undefined>;
} & PublicMethods, {}, {}, {}, vue0.ComponentOptionsMixin, vue0.ComponentOptionsMixin, {
  click: (params: echarts_core0.ECElementEvent) => void;
  dblclick: (params: echarts_core0.ECElementEvent) => void;
  mouseout: (params: echarts_core0.ECElementEvent) => void;
  mouseover: (params: echarts_core0.ECElementEvent) => void;
  mouseup: (params: echarts_core0.ECElementEvent) => void;
  mousedown: (params: echarts_core0.ECElementEvent) => void;
  mousemove: (params: echarts_core0.ECElementEvent) => void;
  contextmenu: (params: echarts_core0.ECElementEvent) => void;
  globalout: (params: echarts_core0.ECElementEvent) => void;
} & {
  highlight: (params: any) => void;
  downplay: (params: any) => void;
  selectchanged: (params: any) => void;
  legendselectchanged: (params: any) => void;
  legendselected: (params: any) => void;
  legendunselected: (params: any) => void;
  legendselectall: (params: any) => void;
  legendinverseselect: (params: any) => void;
  legendscroll: (params: any) => void;
  datazoom: (params: any) => void;
  datarangeselected: (params: any) => void;
  graphroam: (params: any) => void;
  georoam: (params: any) => void;
  treeroam: (params: any) => void;
  timelinechanged: (params: any) => void;
  timelineplaychanged: (params: any) => void;
  restore: (params: any) => void;
  dataviewchanged: (params: any) => void;
  magictypechanged: (params: any) => void;
  geoselectchanged: (params: any) => void;
  geoselected: (params: any) => void;
  geounselected: (params: any) => void;
  axisareaselected: (params: any) => void;
  brush: (params: any) => void;
  brushEnd: (params: any) => void;
  brushselected: (params: any) => void;
  globalcursortaken: (params: any) => void;
} & {
  rendered: (params: {
    elapsedTime: number;
  }) => void;
  finished: () => void;
} & {
  "zr:mousewheel": (params: echarts_core0.ElementEvent) => void;
  "zr:drag": (params: echarts_core0.ElementEvent) => void;
  "zr:dragstart": (params: echarts_core0.ElementEvent) => void;
  "zr:dragend": (params: echarts_core0.ElementEvent) => void;
  "zr:dragenter": (params: echarts_core0.ElementEvent) => void;
  "zr:dragleave": (params: echarts_core0.ElementEvent) => void;
  "zr:dragover": (params: echarts_core0.ElementEvent) => void;
  "zr:drop": (params: echarts_core0.ElementEvent) => void;
  "zr:click": (params: echarts_core0.ElementEvent) => void;
  "zr:dblclick": (params: echarts_core0.ElementEvent) => void;
  "zr:mouseout": (params: echarts_core0.ElementEvent) => void;
  "zr:mouseover": (params: echarts_core0.ElementEvent) => void;
  "zr:mouseup": (params: echarts_core0.ElementEvent) => void;
  "zr:mousedown": (params: echarts_core0.ElementEvent) => void;
  "zr:mousemove": (params: echarts_core0.ElementEvent) => void;
  "zr:contextmenu": (params: echarts_core0.ElementEvent) => void;
  "zr:globalout": (params: echarts_core0.ElementEvent) => void;
}, string, vue0.PublicProps, Readonly<vue0.ExtractPropTypes<{
  loading: BooleanConstructor;
  loadingOptions: PropType<LoadingOptions>;
  autoresize: PropType<AutoResize>;
  option: PropType<Option>;
  theme: {
    type: PropType<Theme>;
  };
  initOptions: PropType<InitOptions>;
  updateOptions: PropType<UpdateOptions>;
  group: StringConstructor;
  manualUpdate: BooleanConstructor;
}>> & Readonly<{
  onClick?: ((params: echarts_core0.ECElementEvent) => any) | undefined;
  onDblclick?: ((params: echarts_core0.ECElementEvent) => any) | undefined;
  onMouseout?: ((params: echarts_core0.ECElementEvent) => any) | undefined;
  onMouseover?: ((params: echarts_core0.ECElementEvent) => any) | undefined;
  onMouseup?: ((params: echarts_core0.ECElementEvent) => any) | undefined;
  onMousedown?: ((params: echarts_core0.ECElementEvent) => any) | undefined;
  onMousemove?: ((params: echarts_core0.ECElementEvent) => any) | undefined;
  onContextmenu?: ((params: echarts_core0.ECElementEvent) => any) | undefined;
  onGlobalout?: ((params: echarts_core0.ECElementEvent) => any) | undefined;
  onHighlight?: ((params: any) => any) | undefined;
  onDownplay?: ((params: any) => any) | undefined;
  onSelectchanged?: ((params: any) => any) | undefined;
  onLegendselectchanged?: ((params: any) => any) | undefined;
  onLegendselected?: ((params: any) => any) | undefined;
  onLegendunselected?: ((params: any) => any) | undefined;
  onLegendselectall?: ((params: any) => any) | undefined;
  onLegendinverseselect?: ((params: any) => any) | undefined;
  onLegendscroll?: ((params: any) => any) | undefined;
  onDatazoom?: ((params: any) => any) | undefined;
  onDatarangeselected?: ((params: any) => any) | undefined;
  onGraphroam?: ((params: any) => any) | undefined;
  onGeoroam?: ((params: any) => any) | undefined;
  onTreeroam?: ((params: any) => any) | undefined;
  onTimelinechanged?: ((params: any) => any) | undefined;
  onTimelineplaychanged?: ((params: any) => any) | undefined;
  onRestore?: ((params: any) => any) | undefined;
  onDataviewchanged?: ((params: any) => any) | undefined;
  onMagictypechanged?: ((params: any) => any) | undefined;
  onGeoselectchanged?: ((params: any) => any) | undefined;
  onGeoselected?: ((params: any) => any) | undefined;
  onGeounselected?: ((params: any) => any) | undefined;
  onAxisareaselected?: ((params: any) => any) | undefined;
  onBrush?: ((params: any) => any) | undefined;
  onBrushEnd?: ((params: any) => any) | undefined;
  onBrushselected?: ((params: any) => any) | undefined;
  onGlobalcursortaken?: ((params: any) => any) | undefined;
  "onZr:mousewheel"?: ((params: echarts_core0.ElementEvent) => any) | undefined;
  "onZr:drag"?: ((params: echarts_core0.ElementEvent) => any) | undefined;
  "onZr:dragstart"?: ((params: echarts_core0.ElementEvent) => any) | undefined;
  "onZr:dragend"?: ((params: echarts_core0.ElementEvent) => any) | undefined;
  "onZr:dragenter"?: ((params: echarts_core0.ElementEvent) => any) | undefined;
  "onZr:dragleave"?: ((params: echarts_core0.ElementEvent) => any) | undefined;
  "onZr:dragover"?: ((params: echarts_core0.ElementEvent) => any) | undefined;
  "onZr:drop"?: ((params: echarts_core0.ElementEvent) => any) | undefined;
  "onZr:click"?: ((params: echarts_core0.ElementEvent) => any) | undefined;
  "onZr:dblclick"?: ((params: echarts_core0.ElementEvent) => any) | undefined;
  "onZr:mouseout"?: ((params: echarts_core0.ElementEvent) => any) | undefined;
  "onZr:mouseover"?: ((params: echarts_core0.ElementEvent) => any) | undefined;
  "onZr:mouseup"?: ((params: echarts_core0.ElementEvent) => any) | undefined;
  "onZr:mousedown"?: ((params: echarts_core0.ElementEvent) => any) | undefined;
  "onZr:mousemove"?: ((params: echarts_core0.ElementEvent) => any) | undefined;
  "onZr:contextmenu"?: ((params: echarts_core0.ElementEvent) => any) | undefined;
  "onZr:globalout"?: ((params: echarts_core0.ElementEvent) => any) | undefined;
  onRendered?: ((params: {
    elapsedTime: number;
  }) => any) | undefined;
  onFinished?: (() => any) | undefined;
}>, {
  manualUpdate: boolean;
  loading: boolean;
}, SlotsTypes, {}, {}, string, vue0.ComponentProvideOptions, true, {}, any>;
//#endregion
export { INIT_OPTIONS_KEY, LOADING_OPTIONS_KEY, THEME_KEY, UPDATE_OPTIONS_KEY, _default as default };
//# sourceMappingURL=index.d.ts.map