import { createElement, forwardRef, type SVGProps } from 'react';
import { artwork, type IconName } from './artwork';
import './icons.css';

export interface KeepSaveIconProps extends SVGProps<SVGSVGElement> {
  size?: number | string;
  absoluteStrokeWidth?: boolean;
  title?: string;
}

/** Familiar semantic silhouettes share open rings, rounded paths and compact core details. */
function createIcon(name: IconName) {
  const Icon = forwardRef<SVGSVGElement, KeepSaveIconProps>(function KeepSaveIcon(
    { size = 24, strokeWidth = 1.75, absoluteStrokeWidth = false, color = 'currentColor', className = '', title, children, ...props }, ref,
  ) {
    const labelled = Boolean(title || props['aria-label'] || props['aria-labelledby']);
    const width = Number(size);
    const line = absoluteStrokeWidth && width > 0 ? Number(strokeWidth) * 24 / width : strokeWidth;
    return <svg ref={ref} xmlns="http://www.w3.org/2000/svg" width={size} height={size} viewBox="0 0 24 24"
      fill="none" stroke={color} strokeWidth={line} strokeLinecap="round" strokeLinejoin="round"
      aria-hidden={labelled ? undefined : true} role={labelled ? 'img' : undefined} focusable="false"
      className={`ks-icon ks-icon-${name} ${className}`} {...props}>
      {title && <title>{title}</title>}
      {artwork[name].map(([tag, attrs], index) => createElement(tag, { ...attrs, key: index }))}
      {children}
    </svg>;
  });
  Icon.displayName = `KeepSave${name}`;
  return Icon;
}

export const Activity = /* @__PURE__ */ createIcon('Activity');
export const AlertCircle = /* @__PURE__ */ createIcon('AlertCircle');
export const AlertOctagon = /* @__PURE__ */ createIcon('AlertOctagon');
export const AlertTriangle = /* @__PURE__ */ createIcon('AlertTriangle');
export const AppWindow = /* @__PURE__ */ createIcon('AppWindow');
export const ArrowDown = /* @__PURE__ */ createIcon('ArrowDown');
export const ArrowLeft = /* @__PURE__ */ createIcon('ArrowLeft');
export const ArrowRight = /* @__PURE__ */ createIcon('ArrowRight');
export const ArrowUpRight = /* @__PURE__ */ createIcon('ArrowUpRight');
export const Ban = /* @__PURE__ */ createIcon('Ban');
export const BarChart3 = /* @__PURE__ */ createIcon('BarChart3');
export const BookOpen = /* @__PURE__ */ createIcon('BookOpen');
export const Bot = /* @__PURE__ */ createIcon('Bot');
export const Boxes = /* @__PURE__ */ createIcon('Boxes');
export const Brain = /* @__PURE__ */ createIcon('Brain');
export const Building2 = /* @__PURE__ */ createIcon('Building2');
export const Cable = /* @__PURE__ */ createIcon('Cable');
export const Calendar = /* @__PURE__ */ createIcon('Calendar');
export const Check = /* @__PURE__ */ createIcon('Check');
export const CheckCircle = /* @__PURE__ */ createIcon('CheckCircle');
export const CheckCircle2 = /* @__PURE__ */ createIcon('CheckCircle2');
export const ChevronDown = /* @__PURE__ */ createIcon('ChevronDown');
export const ChevronRight = /* @__PURE__ */ createIcon('ChevronRight');
export const ChevronUp = /* @__PURE__ */ createIcon('ChevronUp');
export const Clock = /* @__PURE__ */ createIcon('Clock');
export const Code2 = /* @__PURE__ */ createIcon('Code2');
export const Copy = /* @__PURE__ */ createIcon('Copy');
export const CornerDownLeft = /* @__PURE__ */ createIcon('CornerDownLeft');
export const Download = /* @__PURE__ */ createIcon('Download');
export const Eye = /* @__PURE__ */ createIcon('Eye');
export const EyeOff = /* @__PURE__ */ createIcon('EyeOff');
export const FileCode2 = /* @__PURE__ */ createIcon('FileCode2');
export const FolderClosed = /* @__PURE__ */ createIcon('FolderClosed');
export const FolderKey = /* @__PURE__ */ createIcon('FolderKey');
export const FolderOpen = /* @__PURE__ */ createIcon('FolderOpen');
export const Gauge = /* @__PURE__ */ createIcon('Gauge');
export const GitBranch = /* @__PURE__ */ createIcon('GitBranch');
export const GitCompare = /* @__PURE__ */ createIcon('GitCompare');
export const GitPullRequest = /* @__PURE__ */ createIcon('GitPullRequest');
export const History = /* @__PURE__ */ createIcon('History');
export const Home = /* @__PURE__ */ createIcon('Home');
export const Info = /* @__PURE__ */ createIcon('Info');
export const Key = /* @__PURE__ */ createIcon('Key');
export const KeyRound = /* @__PURE__ */ createIcon('KeyRound');
export const Layers = /* @__PURE__ */ createIcon('Layers');
export const LayoutDashboard = /* @__PURE__ */ createIcon('LayoutDashboard');
export const LayoutGrid = /* @__PURE__ */ createIcon('LayoutGrid');
export const Lightbulb = /* @__PURE__ */ createIcon('Lightbulb');
export const List = /* @__PURE__ */ createIcon('List');
export const LoaderCircle = /* @__PURE__ */ createIcon('LoaderCircle');
export const Lock = /* @__PURE__ */ createIcon('Lock');
export const LockKeyhole = /* @__PURE__ */ createIcon('LockKeyhole');
export const LogOut = /* @__PURE__ */ createIcon('LogOut');
export const Menu = /* @__PURE__ */ createIcon('Menu');
export const MessageCircle = /* @__PURE__ */ createIcon('MessageCircle');
export const MessageSquare = /* @__PURE__ */ createIcon('MessageSquare');
export const MessageSquarePlus = /* @__PURE__ */ createIcon('MessageSquarePlus');
export const Moon = /* @__PURE__ */ createIcon('Moon');
export const OrbitHub = /* @__PURE__ */ createIcon('OrbitHub');
export const PanelLeftClose = /* @__PURE__ */ createIcon('PanelLeftClose');
export const PanelLeftOpen = /* @__PURE__ */ createIcon('PanelLeftOpen');
export const Pause = /* @__PURE__ */ createIcon('Pause');
export const Pencil = /* @__PURE__ */ createIcon('Pencil');
export const Play = /* @__PURE__ */ createIcon('Play');
export const Plus = /* @__PURE__ */ createIcon('Plus');
export const Power = /* @__PURE__ */ createIcon('Power');
export const PowerOff = /* @__PURE__ */ createIcon('PowerOff');
export const Puzzle = /* @__PURE__ */ createIcon('Puzzle');
export const Radio = /* @__PURE__ */ createIcon('Radio');
export const RefreshCw = /* @__PURE__ */ createIcon('RefreshCw');
export const RotateCcw = /* @__PURE__ */ createIcon('RotateCcw');
export const Search = /* @__PURE__ */ createIcon('Search');
export const Send = /* @__PURE__ */ createIcon('Send');
export const Server = /* @__PURE__ */ createIcon('Server');
export const Settings = /* @__PURE__ */ createIcon('Settings');
export const Shield = /* @__PURE__ */ createIcon('Shield');
export const ShieldAlert = /* @__PURE__ */ createIcon('ShieldAlert');
export const ShieldCheck = /* @__PURE__ */ createIcon('ShieldCheck');
export const Star = /* @__PURE__ */ createIcon('Star');
export const Sun = /* @__PURE__ */ createIcon('Sun');
export const Terminal = /* @__PURE__ */ createIcon('Terminal');
export const Timer = /* @__PURE__ */ createIcon('Timer');
export const Trash2 = /* @__PURE__ */ createIcon('Trash2');
export const TrendingDown = /* @__PURE__ */ createIcon('TrendingDown');
export const TrendingUp = /* @__PURE__ */ createIcon('TrendingUp');
export const Upload = /* @__PURE__ */ createIcon('Upload');
export const UserRound = /* @__PURE__ */ createIcon('UserRound');
export const Users = /* @__PURE__ */ createIcon('Users');
export const Wrench = /* @__PURE__ */ createIcon('Wrench');
export const X = /* @__PURE__ */ createIcon('X');
export const XCircle = /* @__PURE__ */ createIcon('XCircle');
