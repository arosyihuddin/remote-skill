import Gio from 'gi://Gio';
import Meta from 'gi://Meta';

const DBusIface = `
<node>
  <interface name="org.gnome.Shell.Extensions.Windows">
    <method name="List">
      <arg type="s" direction="out"/>
    </method>
  </interface>
</node>`;

const listWindows = () => {
    const display = global.display;

    if (typeof display?.list_all_windows === 'function') {
        return display.list_all_windows()
            .filter(m => m.get_window_type() !== Meta.WindowType.DESKTOP)
            .map(m => {
                const r = m.get_frame_rect();
                return {
                    id: m.get_id(),
                    title: m.get_title(),
                    wm_class: m.get_wm_class() || '',
                    pid: m.get_pid(),
                    x: r.x, y: r.y,
                    width: r.width, height: r.height,
                    focused: m.has_focus()
                };
            });
    }

    const actors = global.compositor?.get_window_actors
        ? global.compositor.get_window_actors()
        : global.get_window_actors?.() ?? [];
    return actors
        .filter(a => a.meta_window && a.meta_window.get_window_type() !== Meta.WindowType.DESKTOP)
        .map(a => {
            const m = a.meta_window;
            const r = m.get_frame_rect();
            return {
                id: m.get_id(),
                title: m.get_title(),
                wm_class: m.get_wm_class() || '',
                pid: m.get_pid(),
                x: r.x, y: r.y,
                width: r.width, height: r.height,
                focused: m.has_focus()
            };
        });
};

export default class RskWindowsExtension {
    enable() {
        this._dbus = Gio.DBusExportedObject.wrapJSObject(DBusIface, {
            List: () => JSON.stringify(listWindows()),
        });
        this._dbus.export(Gio.DBus.session, '/org/gnome/Shell/Extensions/Windows');
    }

    disable() {
        this._dbus?.unexport();
    }
}
