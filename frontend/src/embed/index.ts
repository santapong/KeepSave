export { KeepSaveWidget, register } from './keepsave-widget';
export { KeepSaveAPI } from './api';
export type { Secret } from './api';
export type { AuthMessage, AuthRequestMessage } from './auth';
export type { WidgetMode } from './widget';

import { register } from './keepsave-widget';

register();
