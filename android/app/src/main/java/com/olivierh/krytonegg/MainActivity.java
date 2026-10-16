package com.olivierh.krytonegg;

import android.app.Activity;
import android.os.Build;
import android.os.Bundle;
import android.view.InputDevice;
import android.view.KeyEvent;
import android.view.View;
import android.view.WindowInsets;
import android.view.WindowInsetsController;
import android.view.WindowManager;
import android.window.OnBackInvokedDispatcher;

import com.olivierh.krytonegg.mobile.EbitenView;
import com.olivierh.krytonegg.mobile.Mobile;

/** Android lifecycle bridge; gameplay, audio, rendering and touch input stay in Go. */
public final class MainActivity extends Activity {
    private EbitenView ebitenView;

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);

        // Android owns the application-private storage path. EbitenView itself
        // registers the JNI context when it initializes the engine.
        Mobile.configure(getFilesDir().getAbsolutePath());
        if (getIntent().hasExtra("krytonegg_verify_ticks")) {
            Mobile.configureVerification(
                    getIntent().getIntExtra("krytonegg_verify_ticks", 0),
                    getIntent().getIntExtra("krytonegg_verify_combat", -1));
        }

        getWindow().addFlags(WindowManager.LayoutParams.FLAG_KEEP_SCREEN_ON);
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.P) {
            WindowManager.LayoutParams attributes = getWindow().getAttributes();
            attributes.layoutInDisplayCutoutMode =
                    WindowManager.LayoutParams.LAYOUT_IN_DISPLAY_CUTOUT_MODE_SHORT_EDGES;
            getWindow().setAttributes(attributes);
        }
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
            getOnBackInvokedDispatcher().registerOnBackInvokedCallback(
                    OnBackInvokedDispatcher.PRIORITY_DEFAULT, this::handleBack);
        }

        ebitenView = new EbitenView(this);
        ebitenView.setFocusableInTouchMode(true);
        ebitenView.requestFocus();
        setContentView(ebitenView);
        hideSystemUi();
    }

    @Override
    protected void onPause() {
        if (ebitenView != null) {
            // Queue the pause on the game thread before suspending its surface.
            Mobile.requestPause();
            ebitenView.suspendGame();
        }
        super.onPause();
    }

    @Override
    protected void onResume() {
        super.onResume();
        hideSystemUi();
        if (ebitenView != null) {
            ebitenView.resumeGame();
        }
    }

    @Override
    public void onBackPressed() {
        handleBack();
    }

    @Override
    public boolean dispatchKeyEvent(KeyEvent event) {
        // EbitenView consumes all legacy key events. Route the older Android
        // system Back key before it reaches the view, preserving gamepad input.
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.TIRAMISU
                && event.getKeyCode() == KeyEvent.KEYCODE_BACK
                && !event.isFromSource(InputDevice.SOURCE_GAMEPAD)
                && !event.isFromSource(InputDevice.SOURCE_JOYSTICK)) {
            if (event.getAction() == KeyEvent.ACTION_UP && !event.isCanceled()) {
                handleBack();
            }
            return true;
        }
        return super.dispatchKeyEvent(event);
    }

    private void handleBack() {
        // The Go application handles pausing and returning to its title screen;
        // Android closes the activity once that title screen is already visible.
        if (Mobile.isAtTitle()) {
            finish();
        } else {
            Mobile.requestBack();
        }
    }

    @Override
    public void onWindowFocusChanged(boolean hasFocus) {
        super.onWindowFocusChanged(hasFocus);
        if (hasFocus) {
            hideSystemUi();
        }
    }

    private void hideSystemUi() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.R) {
            getWindow().setDecorFitsSystemWindows(false);
            WindowInsetsController controller =
                    getWindow().getDecorView().getWindowInsetsController();
            if (controller != null) {
                controller.hide(WindowInsets.Type.statusBars() | WindowInsets.Type.navigationBars());
                controller.setSystemBarsBehavior(
                        WindowInsetsController.BEHAVIOR_SHOW_TRANSIENT_BARS_BY_SWIPE);
            }
            return;
        }
        getWindow().getDecorView().setSystemUiVisibility(
                View.SYSTEM_UI_FLAG_FULLSCREEN
                        | View.SYSTEM_UI_FLAG_HIDE_NAVIGATION
                        | View.SYSTEM_UI_FLAG_IMMERSIVE_STICKY
                        | View.SYSTEM_UI_FLAG_LAYOUT_FULLSCREEN
                        | View.SYSTEM_UI_FLAG_LAYOUT_HIDE_NAVIGATION
                        | View.SYSTEM_UI_FLAG_LAYOUT_STABLE);
    }
}
