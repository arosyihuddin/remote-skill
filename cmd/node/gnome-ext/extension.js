import Gio from 'gi://Gio';
import Meta from 'gi://Meta';

const DBusIface = `
<node>
  <interface name="org.gnome.Shell.Extensions.Windows">
    <method name="List">
      <arg type="s" direction="out"/>
    </method>
    <method name="GetCursorPos">
      <arg type="s" direction="out"/>
    </method>
  </interface>
</node>`;

const listWindows = () => {
    const display = global.display;

    const mapWindow = (m) => {
        const fr = m.get_frame_rect();
        let bx = fr.x, by = fr.y, bw = fr.width, bh = fr.height;
        try {
            const br = m.get_buffer_rect();
            if (br.width > 0 && br.height > 0) {
                bx = br.x; by = br.y;
                bw = br.width; bh = br.height;
            }
        } catch (_) {}
        return {
            id: m.get_id(),
            title: m.get_title(),
            wm_class: m.get_wm_class() || '',
            pid: m.get_pid(),
            x: fr.x, y: fr.y,
            buf_x: bx, buf_y: by,
            width: bw, height: bh,
            frame_w: fr.width, frame_h: fr.height,
            focused: m.has_focus()
        };
    };

    if (typeof display?.list_all_windows === 'function') {
        return display.list_all_windows()
            .filter(m => m.get_window_type() !== Meta.WindowType.DESKTOP)
            .map(mapWindow);
    }

    const actors = global.compositor?.get_window_actors
        ? global.compositor.get_window_actors()
        : global.get_window_actors?.() ?? [];
    return actors
        .filter(a => a.meta_window && a.meta_window.get_window_type() !== Meta.WindowType.DESKTOP)
        .map(a => mapWindow(a.meta_window));
};

export default class RskWindowsExtension {
    enable() {
        this._dbus = Gio.DBusExportedObject.wrapJSObject(DBusIface, {
            List: () => JSON.stringify(listWindows()),
            GetCursorPos: () => {
                const [x, y] = global.get_pointer();
                return JSON.stringify({ x, y });
            },
        });
        this._dbus.export(Gio.DBus.session, '/org/gnome/Shell/Extensions/Windows');
    }

    disable() {
        this._dbus?.unexport();
    }
}
