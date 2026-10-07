// Recover only reference evidence; the Go game never executes this program.
// @category Krytonegg
import ghidra.app.script.GhidraScript;
import ghidra.app.decompiler.DecompInterface;
import ghidra.app.decompiler.DecompileResults;
import ghidra.program.model.listing.Function;
import ghidra.program.model.listing.Instruction;
import java.io.PrintWriter;

public class VerifyOriginal extends GhidraScript {
    public void run() throws Exception {
        String output = getScriptArgs()[0];
        long[] entries = {0x603c, 0x11ea, 0x265c};
        String[] names = {"UpdateOriginalCombat", "QueueOriginalPaulaEvent", "UpdateOriginalEnemies"};
        PrintWriter writer = new PrintWriter(output);
        DecompInterface decompiler = new DecompInterface();
        decompiler.openProgram(currentProgram);
        for (int index = 0; index < entries.length; index++) {
            disassemble(toAddr(entries[index]));
            Function function = getFunctionAt(toAddr(entries[index]));
            if (function == null) function = createFunction(toAddr(entries[index]), names[index]);
            writer.println("\n// Entry " + Long.toHexString(entries[index]) + ": " + names[index]);
            if (function != null) {
                DecompileResults result = decompiler.decompileFunction(function, 30, monitor);
                if (result.decompileCompleted()) writer.println(result.getDecompiledFunction().getC());
                else writer.println(result.getErrorMessage());
            }
        }
        writer.close();
        decompiler.dispose();
    }
}
