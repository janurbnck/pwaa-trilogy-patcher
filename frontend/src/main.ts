import { Dialogs, Events } from "@wailsio/runtime";
import { PatcherService } from "../bindings/changeme";

const gameFolder = document.getElementById("game_folder") as HTMLInputElement;
const btnSearch = document.getElementById("btn_search") as HTMLButtonElement;
const detectedVersion = document.getElementById("detected_version") as HTMLInputElement;
const versionCombo = document.getElementById("version_combo") as HTMLSelectElement;
const btnAction = document.getElementById("btn_action") as HTMLButtonElement;

let patcherDisplayText: Record<string, string | undefined> = {};
let currentVersion = "";

function setDetectedVersion(versionKey: string) {
    currentVersion = versionKey;
    detectedVersion.value = patcherDisplayText[versionKey] ?? versionKey;
    updateActionState();
}

function updateActionState() {
    btnAction.textContent = versionCombo.value === "orig" ? "Unpatch" : "Patch";
    btnAction.disabled =
        !gameFolder.value.trim() ||
        !currentVersion ||
        currentVersion === "error" ||
        currentVersion === versionCombo.value;
}

async function setFolder(folder: string) {
    if (!folder) return;
    if (Object.keys(patcherDisplayText).length === 0) {
        patcherDisplayText = (await PatcherService.GetDisplayText()) ?? {};
    }
    gameFolder.value = folder;
    setDetectedVersion(await PatcherService.DetectVersion(folder));
}

btnSearch.addEventListener("click", async () => {
    await setFolder(await PatcherService.SelectGameFolder());
});

versionCombo.addEventListener("change", updateActionState);

btnAction.addEventListener("click", async () => {
    const folder = gameFolder.value.trim();
    if (!folder || currentVersion === "error") {
        void Dialogs.Warning({ Title: "Chyba", Message: "Vyberte prosím platný herní adresář." });
        return;
    }

    const version = versionCombo.value;
    btnAction.disabled = true;
    try {
        if (version === "orig") {
            const res = await PatcherService.Unpatch(folder);
            if (res.code === 0) {
                setDetectedVersion("orig");
                void Dialogs.Info({ Title: "Úspěch", Message: res.message });
            } else {
                void Dialogs.Error({ Title: "Chyba", Message: res.message });
            }
        } else {
            const res = await PatcherService.Patch(folder, version);
            if (res.code === 0) {
                setDetectedVersion(version);
                void Dialogs.Info({ Title: "Úspěch", Message: res.message });
            } else if (res.code === 101) {
                void Dialogs.Info({ Title: "Patch není třeba", Message: res.message });
            } else {
                void Dialogs.Error({ Title: "Chyba", Message: res.message });
            }
        }
    } finally {
        await setFolder(folder);
        updateActionState();
    }
});

Events.On(Events.Types.Common.WindowFilesDropped, (e) => {
    void setFolder(e?.data?.files?.[0]);
});

(async () => {
    patcherDisplayText = (await PatcherService.GetDisplayText()) ?? {};
    const versions = (await PatcherService.GetAvailableVersions()) ?? [];
    versionCombo.innerHTML = versions
        .map((v) => `<option value="${v}">${patcherDisplayText[v] ?? v}</option>`)
        .join("");
    updateActionState();
})();
